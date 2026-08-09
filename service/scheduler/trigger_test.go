package scheduler

import "testing"

func TestValidateTriggerSource(t *testing.T) {
	for _, src := range []string{"gmail", "whatsapp", "telegram", "imessage", "applenotes"} {
		if err := ValidateTriggerSource(src); err != nil {
			t.Errorf("ValidateTriggerSource(%q) = %v, want nil", src, err)
		}
	}
	if err := ValidateTriggerSource("unknown"); err == nil {
		t.Error("expected error for unknown source")
	}
}

func TestValidateTriggerQuery(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		query   string
		wantErr bool
	}{
		{"valid simple", "gmail", "from_addr LIKE '%@acme.com%'", false},
		{"valid AND", "gmail", "from_addr = 'x' AND subject LIKE '%urgent%'", false},
		{"valid OR parens", "gmail", "(from_addr LIKE '%@a.com%') OR (subject LIKE '%b%')", false},
		{"valid IS NOT NULL", "gmail", "from_addr IS NOT NULL", false},
		{"valid IN", "gmail", "from_addr IN ('a@b.com', 'c@d.com')", false},
		{"valid whatsapp", "whatsapp", "sender_name = 'Alice' AND text LIKE '%meeting%'", false},
		{"valid telegram", "telegram", "sender_name = 'Alice' AND text LIKE '%meeting%'", false},
		{"telegram unknown column", "telegram", "chat_jid = 'x'", true},
		{"empty", "gmail", "", true},
		{"semicolon", "gmail", "1=1; DROP TABLE emails", true},
		{"comment", "gmail", "from_addr = 'x' -- comment", true},
		{"unbalanced parens", "gmail", "from_addr LIKE '(' AND (subject = 'x'", true},

		// SQL injection attempts — blocked by allowlist
		{"union select", "gmail", "1=1 UNION SELECT sql FROM sqlite_master", true},
		{"subquery", "gmail", "from_addr IN (SELECT from_addr FROM emails)", true},
		{"drop", "gmail", "DROP TABLE emails", true},
		{"delete", "gmail", "DELETE FROM emails", true},
		{"insert", "gmail", "INSERT INTO emails VALUES(1)", true},
		{"update", "gmail", "UPDATE emails SET x=1", true},
		{"attach", "gmail", "ATTACH DATABASE '/tmp/x' AS x", true},
		{"pragma", "gmail", "PRAGMA table_info(emails)", true},
		{"load_extension", "gmail", "load_extension('/tmp/evil')", true},
		{"unknown column", "gmail", "nonexistent_col = 'x'", true},
		{"function call", "gmail", "lower(from_addr) = 'x'", true},

		// Keywords inside string literals are safe
		{"keyword in string", "gmail", "subject LIKE '%DROP TABLE%'", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTriggerQuery(tt.source, tt.query)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTriggerQuery(%q, %q) = %v, wantErr %v", tt.source, tt.query, err, tt.wantErr)
			}
		})
	}
}

func TestBuildTriggerQuery(t *testing.T) {
	q, args, err := BuildTriggerQuery("gmail", "from_addr LIKE '%@test.com%'", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(args) != 1 || args[0] != int64(5) {
		t.Errorf("args = %v, want [5]", args)
	}
	if q == "" {
		t.Error("expected non-empty query")
	}

	_, _, err = BuildTriggerQuery("nonexistent", "x=1", 0)
	if err == nil {
		t.Error("expected error for unknown source")
	}
}

func TestCheckTrigger(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	// Create a test emails table matching the gmail trigger template.
	_, err := db.Exec(`CREATE TABLE emails (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		subject TEXT,
		from_addr TEXT,
		date TEXT
	)`)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(`INSERT INTO emails (subject, from_addr, date) VALUES
		('Newsletter', 'news@example.com', '2026-01-01'),
		('Q1 Planning', 'boss@acme.com', '2026-01-02'),
		('Ticket resolved', 'support@vendor.io', '2026-01-03')`)
	if err != nil {
		t.Fatal(err)
	}

	match, err := CheckTrigger(db, "gmail", "from_addr LIKE '%@acme.com%'", 0)
	if err != nil {
		t.Fatal(err)
	}
	if match == nil {
		t.Fatal("expected match, got nil")
	}
	if len(match.Rows) != 1 {
		t.Errorf("got %d rows, want 1", len(match.Rows))
	}
	if match.MaxID != 2 {
		t.Errorf("MaxID = %d, want 2", match.MaxID)
	}

	// Watermark past the match should return nil.
	match, err = CheckTrigger(db, "gmail", "from_addr LIKE '%@acme.com%'", 2)
	if err != nil {
		t.Fatal(err)
	}
	if match != nil {
		t.Errorf("expected nil match after watermark, got %d rows", len(match.Rows))
	}
}

