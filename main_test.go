package main

import "testing"

func TestTargetPaneID(t *testing.T) {
	cases := []struct {
		name, ctx, env, want string
	}{
		{"context wins", `{"focused_pane_id":"w1:p2","workspace_id":"w1"}`, "w9:p9", "w1:p2"},
		{"empty context", "", "w3:p5", "w3:p5"},
		{"context without pane", `{"workspace_id":"w1"}`, "w3:p5", "w3:p5"},
		{"bad json", `{`, "w3:p5", "w3:p5"},
		{"nothing", "", "", ""},
	}
	for _, c := range cases {
		if got := targetPaneID(c.ctx, c.env); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestParsePaneGet(t *testing.T) {
	out := `{"id":"cli:pane:get","result":{"pane":{"agent_status":"unknown","cwd":"C:\\Users\\Administrator\\","foreground_cwd":"F:\\GoProject","pane_id":"w3:p2"},"type":"pane_info"}}`
	info, err := parsePaneGet([]byte(out))
	if err != nil {
		t.Fatal(err)
	}
	if info.CWD != `C:\Users\Administrator\` || info.ForegroundCWD != `F:\GoProject` {
		t.Fatalf("unexpected info: %+v", info)
	}

	if _, err := parsePaneGet([]byte(`{"id":"x","error":{"code":"pane_not_found","message":"no pane"}}`)); err == nil {
		t.Fatal("expected error for herdr error response")
	}
	if _, err := parsePaneGet([]byte(`{"result":{}}`)); err == nil {
		t.Fatal("expected error for missing pane")
	}
	if _, err := parsePaneGet([]byte(`not json`)); err == nil {
		t.Fatal("expected error for invalid json")
	}
}

func TestPickCWD(t *testing.T) {
	if got := pickCWD(paneInfo{CWD: `F:\Project\abc\`, ForegroundCWD: `C:\other`}); got != `F:\Project\abc` {
		t.Errorf("cwd preferred: got %q", got)
	}
	if got := pickCWD(paneInfo{ForegroundCWD: `C:\other`}); got != `C:\other` {
		t.Errorf("foreground fallback: got %q", got)
	}
	if got := pickCWD(paneInfo{}); got != "" {
		t.Errorf("empty: got %q", got)
	}
}

func TestCleanPath(t *testing.T) {
	cases := map[string]string{
		`C:\Users\Administrator\`: `C:\Users\Administrator`,
		`C:\`:                     `C:\`,
		`F:\Project\abc`:          `F:\Project\abc`,
		`/home/user/`:             `/home/user`,
		`/`:                       `/`,
		`  D:\x\\  `:              `D:\x`,
		``:                        ``,
	}
	for in, want := range cases {
		if got := cleanPath(in); got != want {
			t.Errorf("cleanPath(%q) = %q, want %q", in, got, want)
		}
	}
}
