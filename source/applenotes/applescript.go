package applenotes

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const fieldSep = "%%FIELD%%"
const recordSep = "%%RECORD%%"

func runAppleScript(script string) (string, error) {
	cmd := exec.Command("osascript", "-e", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("osascript: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return strings.TrimSpace(string(out)), nil
}

// FetchAllNotes fetches all notes from Apple Notes using batch property access.
// Returns notes without folder mapping (folder info is added by FetchFolders).
func FetchAllNotes() ([]Note, error) {
	script := fmt.Sprintf(`
tell application "Notes"
	set noteIDs to id of every note
	set noteNames to name of every note
	set noteDates to modification date of every note
	set noteCreated to creation date of every note
	set noteProtected to password protected of every note
	set notePlain to plaintext of every note

	set output to ""
	repeat with i from 1 to count of noteIDs
		set nID to item i of noteIDs
		set nName to item i of noteNames
		set nMod to item i of noteDates
		set nCreated to item i of noteCreated
		set nProt to item i of noteProtected
		set nText to item i of notePlain

		set output to output & nID & "%s" & nName & "%s" & nMod & "%s" & nCreated & "%s" & nProt & "%s" & nText & "%s"
	end repeat
	return output
end tell`, fieldSep, fieldSep, fieldSep, fieldSep, fieldSep, recordSep)

	out, err := runAppleScript(script)
	if err != nil {
		return nil, fmt.Errorf("fetch all notes: %w", err)
	}

	if out == "" {
		return nil, nil
	}

	records := strings.Split(out, recordSep)
	var notes []Note
	for _, rec := range records {
		rec = strings.TrimSpace(rec)
		if rec == "" {
			continue
		}
		fields := strings.Split(rec, fieldSep)
		if len(fields) < 6 {
			continue
		}

		n := Note{
			AppleID:           strings.TrimSpace(fields[0]),
			Title:             strings.TrimSpace(fields[1]),
			PasswordProtected: strings.TrimSpace(fields[4]) == "true",
			Body:              strings.TrimSpace(fields[5]),
		}
		n.ModifiedAt = parseAppleScriptDate(strings.TrimSpace(fields[2]))
		n.CreatedAt = parseAppleScriptDate(strings.TrimSpace(fields[3]))

		notes = append(notes, n)
	}
	return notes, nil
}

// FetchFolders fetches all folders and their note IDs from Apple Notes.
// Returns a map of noteAppleID -> Folder.
func FetchFolders() ([]Folder, map[string]Folder, error) {
	script := fmt.Sprintf(`
tell application "Notes"
	set output to ""
	repeat with f in every folder
		set fID to id of f
		set fName to name of f
		try
			set parentClass to class of container of f
			if parentClass is folder then
				set parentID to id of container of f
			else
				set parentID to ""
			end if
		on error
			set parentID to ""
		end try
		try
			set acctName to name of container of f
		on error
			set acctName to ""
		end try

		set noteIDs to id of notes of f
		set noteIDStr to ""
		repeat with nID in noteIDs
			set noteIDStr to noteIDStr & nID & ","
		end repeat

		set output to output & fID & "%s" & fName & "%s" & parentID & "%s" & acctName & "%s" & noteIDStr & "%s"
	end repeat
	return output
end tell`, fieldSep, fieldSep, fieldSep, fieldSep, recordSep)

	out, err := runAppleScript(script)
	if err != nil {
		return nil, nil, fmt.Errorf("fetch folders: %w", err)
	}

	if out == "" {
		return nil, nil, nil
	}

	records := strings.Split(out, recordSep)
	var folders []Folder
	noteToFolder := make(map[string]Folder)

	for _, rec := range records {
		rec = strings.TrimSpace(rec)
		if rec == "" {
			continue
		}
		fields := strings.Split(rec, fieldSep)
		if len(fields) < 5 {
			continue
		}

		f := Folder{
			AppleID:       strings.TrimSpace(fields[0]),
			Name:          strings.TrimSpace(fields[1]),
			ParentAppleID: strings.TrimSpace(fields[2]),
			Account:       strings.TrimSpace(fields[3]),
		}

		// Skip "Recently Deleted" folder
		if isRecentlyDeletedFolder(f.Name) {
			continue
		}

		folders = append(folders, f)

		// Map each note ID to this folder
		noteIDsStr := strings.TrimSpace(fields[4])
		if noteIDsStr != "" {
			for _, nID := range strings.Split(noteIDsStr, ",") {
				nID = strings.TrimSpace(nID)
				if nID != "" {
					noteToFolder[nID] = f
				}
			}
		}
	}

	return folders, noteToFolder, nil
}

// recentlyDeletedNames contains "Recently Deleted" in various languages.
var recentlyDeletedNames = map[string]bool{
	"recently deleted":        true,
	"récemment supprimées":    true,
	"zuletzt gelöscht":        true,
	"eliminadas recientemente": true,
	"最近削除した項目":                true,
	"최근 삭제한 항목":               true,
	"最近删除":                    true,
}

func isRecentlyDeletedFolder(name string) bool {
	return recentlyDeletedNames[strings.ToLower(name)]
}

func parseAppleScriptDate(s string) time.Time {
	// AppleScript date format varies by locale. Common formats:
	formats := []string{
		"Monday, January 2, 2006 at 3:04:05 PM",
		"Monday, January 2, 2006 at 3:04:05\u202fPM",
		"Monday, 2 January 2006 at 15:04:05",
		"2006-01-02 15:04:05 -0700",
		time.RFC3339,
		"1/2/2006 3:04:05 PM",
		"2/1/2006 3:04:05 PM",
		"January 2, 2006 at 3:04:05 PM",
		"January 2, 2006 at 3:04:05\u202fPM",
		"2 January 2006 at 15:04:05",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t
		}
	}
	return time.Time{}
}
