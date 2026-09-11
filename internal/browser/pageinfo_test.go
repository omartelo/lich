package browser

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPageInfoScriptNeverSerializesControlValues(t *testing.T) {
	if !strings.Contains(pageInfoScript, "data-lich-idx") {
		t.Fatal("page-info must stamp data-lich-idx so click/type can find the node")
	}
	if strings.Contains(pageInfoScript, "data-nim-idx") {
		t.Fatal("page-info still names the other product's attribute")
	}
	if strings.Contains(pageInfoScript, "innerText || el.value") || strings.Contains(pageInfoScript, "innerText || value") {
		t.Fatal("innerText || value is how a password reaches the transcript")
	}
	if !strings.Contains(pageInfoScript, "entry.filled") {
		t.Fatal("agents need filled, not the value, to know typing landed")
	}
	if !strings.Contains(pageInfoScript, "entry.checked") {
		t.Fatal("agents need checked, not the value, for radios and checkboxes")
	}
}

func TestInteractiveJSONHasNoValueField(t *testing.T) {
	filled, checked := true, false
	el := Interactive{
		Index: 0, Tag: "input", Type: "password", Text: "Password",
		Filled: &filled, Checked: &checked,
	}
	raw, err := json.Marshal(el)
	if err != nil {
		t.Fatal(err)
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(raw, &keys); err != nil {
		t.Fatal(err)
	}
	if _, ok := keys["value"]; ok {
		t.Fatalf("interactive JSON leaked a value field: %s", raw)
	}
	body, err := json.Marshal(PageInfo{URL: "https://ex", Interactive: []Interactive{el}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), `"value"`) {
		t.Fatalf("page-info JSON leaked a value key: %s", body)
	}
}
