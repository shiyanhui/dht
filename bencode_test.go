package dht

import (
	"testing"
)

func TestDecodeString(t *testing.T) {
	cases := []struct {
		in  string
		out string
	}{
		{"0:", ""},
		{"1:a", "a"},
		{"5:hello", "hello"},
	}

	for _, c := range cases {
		if out, err := Decode([]byte(c.in)); err != nil || out != c.out {
			t.Error(err)
		}
	}
}

func TestDecodeInt(t *testing.T) {
	cases := []struct {
		in  string
		out int
	}{
		{"i123e:", 123},
		{"i0e", 0},
		{"i-1e", -1},
	}

	for _, c := range cases {
		if out, err := Decode([]byte(c.in)); err != nil || out != c.out {
			t.Error(err)
		}
	}
}

func TestDecodeList(t *testing.T) {
	cases := []struct {
		in  string
		out []interface{}
	}{
		{"li123ei-1ee", []interface{}{123, -1}},
		{"l5:helloe", []interface{}{"hello"}},
		{"ld5:hello5:worldee", []interface{}{map[string]interface{}{"hello": "world"}}},
		{"lli1ei2eee", []interface{}{[]interface{}{1, 2}}},
	}

	for i, c := range cases {
		v, err := Decode([]byte(c.in))
		if err != nil {
			t.Fail()
		}

		out := v.([]interface{})

		switch i {
		case 0, 1:
			for j, item := range out {
				if item != c.out[j] {
					t.Fail()
				}
			}
		case 2:
			if len(out) != 1 {
				t.Fail()
			}

			o := out[0].(map[string]interface{})
			cout := c.out[0].(map[string]interface{})

			for k, v := range o {
				if cv, ok := cout[k]; !ok || v != cv {
					t.Fail()
				}
			}
		case 3:
			if len(out) != 1 {
				t.Fail()
			}

			o := out[0].([]interface{})
			cout := c.out[0].([]interface{})

			for j, item := range o {
				if item != cout[j] {
					t.Fail()
				}
			}
		}
	}
}

func TestDecodeDict(t *testing.T) {
	cases := []struct {
		in  string
		out map[string]interface{}
	}{
		{"d5:helloi100ee", map[string]interface{}{"hello": 100}},
		{"d3:foo3:bare", map[string]interface{}{"foo": "bar"}},
		{"d1:ad3:foo3:baree", map[string]interface{}{"a": map[string]interface{}{"foo": "bar"}}},
		{"d4:listli1eee", map[string]interface{}{"list": []interface{}{1}}},
	}

	for i, c := range cases {
		v, err := Decode([]byte(c.in))
		if err != nil {
			t.Fail()
		}

		out := v.(map[string]interface{})

		switch i {
		case 0, 1:
			for k, v := range out {
				if cv, ok := c.out[k]; !ok || v != cv {
					t.Fail()
				}
			}
		case 2:
			if len(out) != 1 {
				t.Fail()
			}

			v, ok := out["a"]
			if !ok {
				t.Fail()
			}

			cout := c.out["a"].(map[string]interface{})

			for k, v := range v.(map[string]interface{}) {
				if cv, ok := cout[k]; !ok || v != cv {
					t.Fail()
				}
			}
		case 3:
			if len(out) != 1 {
				t.Fail()
			}

			v, ok := out["list"]
			if !ok {
				t.Fail()
			}

			cout := c.out["list"].([]interface{})

			for j, v := range v.([]interface{}) {
				if v != cout[j] {
					t.Fail()
				}
			}
		}
	}
}
