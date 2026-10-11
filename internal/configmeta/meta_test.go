package configmeta

import (
	"reflect"
	"testing"
	"time"
)

type pathServer struct {
	Host string
	Port int
}

type pathConfig struct {
	APIURL    string
	Timeout   time.Duration
	Server    pathServer
	Labels    map[string]string
	Servers   map[string]pathServer
	Durations map[string]time.Duration
	Numbers   []int
	Pair      [2]string
	Empty     map[string]string
}

func TestConcreteLeafPathsDerivesCanonicalPaths(t *testing.T) {
	paths, err := ConcreteLeafPaths(pathConfig{
		Labels: map[string]string{
			"env":   "prod",
			"a.b":   "dot",
			`a"b\c`: "quoted",
		},
		Servers: map[string]pathServer{
			"prod": {Host: "prod.example", Port: 443},
		},
		Durations: map[string]time.Duration{"wait": time.Second},
		Numbers:   []int{1, 2},
		Pair:      [2]string{"a", "b"},
		Empty:     map[string]string{},
	})
	if err != nil {
		t.Fatalf("ConcreteLeafPaths() error = %v", err)
	}

	want := []string{
		"apiurl",
		"timeout",
		"server.host",
		"server.port",
		`labels["a\"b\\c"]`,
		`labels["a.b"]`,
		`labels["env"]`,
		`servers["prod"].host`,
		`servers["prod"].port`,
		`durations["wait"]`,
		"numbers",
		"pair",
	}
	if !reflect.DeepEqual(paths, want) {
		t.Fatalf("ConcreteLeafPaths() = %#v, want %#v", paths, want)
	}
}

func TestConcreteLeafPathsRejectsEmptyMapKeys(t *testing.T) {
	_, err := ConcreteLeafPaths(struct {
		Labels map[string]string
	}{
		Labels: map[string]string{"": "bad"},
	})
	if err == nil {
		t.Fatalf("ConcreteLeafPaths() error = nil")
	}
}

func TestIsKebabCase(t *testing.T) {
	valid := []string{"api-url", "timeout", "server-port", "s3-bucket", "a"}
	for _, value := range valid {
		if !IsKebabCase(value) {
			t.Fatalf("IsKebabCase(%q) = false", value)
		}
	}
	invalid := []string{"", "-", "API_URL", "_api", "api_", "api--url", "api.url", "api-"}
	for _, value := range invalid {
		if IsKebabCase(value) {
			t.Fatalf("IsKebabCase(%q) = true", value)
		}
	}
}

func TestParseTextTrimsAndKeepsLastOccurrences(t *testing.T) {
	got, err := ParseText("a, b,c,a, b ", reflect.TypeOf([]string{}))
	if err != nil {
		t.Fatalf("ParseText() error = %v", err)
	}
	want := []string{"c", "a", "b"}
	if !reflect.DeepEqual(got.Interface(), want) {
		t.Fatalf("ParseText() = %#v, want %#v", got.Interface(), want)
	}

	ints, err := ParseText("1, 2,01,3", reflect.TypeOf([]int{}))
	if err != nil {
		t.Fatalf("ParseText() ints error = %v", err)
	}
	wantInts := []int{2, 1, 3}
	if !reflect.DeepEqual(ints.Interface(), wantInts) {
		t.Fatalf("ParseText() ints = %#v, want %#v", ints.Interface(), wantInts)
	}
}

func TestParseTextKeepsBlankSliceItems(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []string
	}{
		{name: "empty input is an empty slice", text: "", want: []string{}},
		{name: "whitespace-only input is one blank item", text: "  ", want: []string{""}},
		{name: "doubled comma keeps a blank item", text: "work,,laptop", want: []string{"work", "", "laptop"}},
		{name: "repeated blank items keep the last", text: "work,,laptop,", want: []string{"work", "laptop", ""}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseText(tt.text, reflect.TypeOf([]string{}))
			if err != nil {
				t.Fatalf("ParseText(%q) error = %v", tt.text, err)
			}
			if !reflect.DeepEqual(got.Interface(), tt.want) {
				t.Errorf("ParseText(%q) = %#v, want %#v", tt.text, got.Interface(), tt.want)
			}
		})
	}
}
