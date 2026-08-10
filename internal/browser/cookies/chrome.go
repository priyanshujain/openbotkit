package cookies

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/crypto/pbkdf2"
	_ "modernc.org/sqlite"
)

const (
	chromeKeychainService  = "Chrome Safe Storage"
	chromeKeychainAccount  = "Chrome"
	chromePBKDF2Iterations = 1003
	chromePBKDF2KeyLen     = 16

	// Chrome DB version 24+ prepends SHA256(host_key) to the cookie
	// value before encrypting. After decryption we must strip it.
	minHashPrefixVersion = 24
	sha256Len            = sha256.Size // 32
)

func GetChromeEncryptionKey() ([]byte, error) {
	password, err := getChromeKeychainPassword()
	if err != nil {
		return nil, err
	}
	return pbkdf2.Key([]byte(password), []byte("saltysalt"), chromePBKDF2Iterations, chromePBKDF2KeyLen, sha1.New), nil
}

func DecryptChromeValue(encrypted, derivedKey []byte, hashPrefix bool) (string, error) {
	if len(encrypted) < 3 {
		return "", fmt.Errorf("encrypted value too short (%d bytes)", len(encrypted))
	}
	prefix := string(encrypted[:3])
	if prefix != "v10" {
		return "", fmt.Errorf("unsupported encryption version %q", prefix)
	}
	encrypted = encrypted[3:]

	block, err := aes.NewCipher(derivedKey)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}

	if len(encrypted) < aes.BlockSize || len(encrypted)%aes.BlockSize != 0 {
		return "", fmt.Errorf("ciphertext length %d is not a multiple of block size", len(encrypted))
	}

	iv := make([]byte, aes.BlockSize)
	for i := range iv {
		iv[i] = ' '
	}

	mode := cipher.NewCBCDecrypter(block, iv)
	plaintext := make([]byte, len(encrypted))
	mode.CryptBlocks(plaintext, encrypted)

	if len(plaintext) > 0 {
		padLen := int(plaintext[len(plaintext)-1])
		if padLen > 0 && padLen <= aes.BlockSize && padLen <= len(plaintext) {
			valid := true
			for i := 0; i < padLen; i++ {
				if plaintext[len(plaintext)-1-i] != byte(padLen) {
					valid = false
					break
				}
			}
			if valid {
				plaintext = plaintext[:len(plaintext)-padLen]
			}
		}
	}

	if hashPrefix && len(plaintext) >= sha256Len {
		plaintext = plaintext[sha256Len:]
	}

	return string(plaintext), nil
}

func ExtractChromeCookie(hosts []string, names []string) (map[string]string, error) {
	key, err := GetChromeEncryptionKey()
	if err != nil {
		return nil, fmt.Errorf("get chrome encryption key: %w", err)
	}

	profileDirs, err := findChromeProfileDirs()
	if err != nil {
		return nil, err
	}
	if len(profileDirs) == 0 {
		return nil, fmt.Errorf("no Chrome profile directories found")
	}

	var lastErr error
	for _, dir := range profileDirs {
		dbPath := filepath.Join(dir, "Cookies")
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			continue
		}

		result, err := extractChromeCookiesFromDB(dbPath, key, hosts, names)
		if err != nil {
			lastErr = err
			continue
		}
		if len(result) > 0 {
			return result, nil
		}
	}

	if lastErr != nil {
		return nil, fmt.Errorf("chrome cookie extraction failed: %w", lastErr)
	}
	return nil, fmt.Errorf("no matching cookies found in Chrome")
}

func extractChromeCookiesFromDB(dbPath string, key []byte, hosts, names []string) (map[string]string, error) {
	tmp, err := copyToTemp(dbPath, "chrome-cookies-*.db")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmp)

	db, err := sql.Open("sqlite", tmp+"?mode=ro")
	if err != nil {
		return nil, fmt.Errorf("open chrome cookie db: %w", err)
	}
	defer db.Close()

	hashPrefix := chromeDBVersion(db) >= minHashPrefixVersion

	hostPlaceholders := make([]string, len(hosts))
	hostArgs := make([]any, len(hosts))
	for i, h := range hosts {
		hostPlaceholders[i] = "?"
		hostArgs[i] = h
	}

	namePlaceholders := make([]string, len(names))
	nameArgs := make([]any, len(names))
	for i, n := range names {
		namePlaceholders[i] = "?"
		nameArgs[i] = n
	}

	query := fmt.Sprintf(
		`SELECT name, encrypted_value FROM cookies WHERE host_key IN (%s) AND name IN (%s) ORDER BY expires_utc DESC`,
		strings.Join(hostPlaceholders, ","),
		strings.Join(namePlaceholders, ","),
	)

	args := append(hostArgs, nameArgs...)
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query chrome cookies: %w", err)
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var name string
		var encValue []byte
		if err := rows.Scan(&name, &encValue); err != nil {
			continue
		}
		if _, exists := result[name]; exists {
			continue
		}
		value, err := DecryptChromeValue(encValue, key, hashPrefix)
		if err != nil {
			continue
		}
		if value != "" {
			result[name] = value
		}
	}

	return result, nil
}

// chromeDBVersion reads the schema version from Chrome's Cookies database.
func chromeDBVersion(db *sql.DB) int {
	var version int
	err := db.QueryRow(`SELECT value FROM meta WHERE key = 'version'`).Scan(&version)
	if err != nil {
		return 0
	}
	return version
}

func findChromeProfileDirs() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home dir: %w", err)
	}

	var baseDir string
	switch runtime.GOOS {
	case "darwin":
		baseDir = filepath.Join(home, "Library", "Application Support", "Google", "Chrome")
	case "linux":
		baseDir = filepath.Join(home, ".config", "google-chrome")
	default:
		return nil, fmt.Errorf("chrome cookie extraction not supported on %s", runtime.GOOS)
	}

	if _, err := os.Stat(baseDir); os.IsNotExist(err) {
		return nil, nil
	}

	var dirs []string
	dirs = append(dirs, filepath.Join(baseDir, "Default"))

	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return dirs, nil
	}
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "Profile ") {
			dirs = append(dirs, filepath.Join(baseDir, e.Name()))
		}
	}

	return dirs, nil
}

func getChromeKeychainPassword() (string, error) {
	if runtime.GOOS == "linux" {
		out, err := exec.Command("secret-tool", "lookup", "xdg:schema", "chrome_libsecret_os_crypt_password_v2").Output()
		if err == nil && len(strings.TrimSpace(string(out))) > 0 {
			return strings.TrimSpace(string(out)), nil
		}
		out, err = exec.Command("secret-tool", "lookup", "application", "chrome").Output()
		if err == nil && len(strings.TrimSpace(string(out))) > 0 {
			return strings.TrimSpace(string(out)), nil
		}
		// Chrome falls back to "peanuts" when no system keyring is available.
		// See chromium/src/components/os_crypt/sync/os_crypt_linux.cc
		return "peanuts", nil
	}

	if runtime.GOOS != "darwin" {
		return "", fmt.Errorf("keychain access only supported on macOS and Linux")
	}

	out, err := exec.Command("security", "find-generic-password",
		"-s", chromeKeychainService,
		"-a", chromeKeychainAccount,
		"-w",
	).Output()
	if err != nil {
		return "", fmt.Errorf("security command: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func copyToTemp(src, pattern string) (string, error) {
	data, err := os.ReadFile(src)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", src, err)
	}
	f, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", fmt.Errorf("create temp: %w", err)
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", fmt.Errorf("write temp: %w", err)
	}
	f.Close()
	return f.Name(), nil
}
