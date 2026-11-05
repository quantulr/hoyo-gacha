package main

import (
	"encoding/json"
	"testing"
)

func TestGachaGroupMarshal(t *testing.T) {
	empty, err := json.Marshal(gachaGroup{Type: "1"})
	if err != nil {
		t.Fatal(err)
	}
	if string(empty) != `["1",[]]` {
		t.Errorf("空频段 = %s", empty)
	}
	full, err := json.Marshal(gachaGroup{Type: "2", Records: []gachaRecord{{ID: "1"}}})
	if err != nil {
		t.Fatal(err)
	}
	if string(full) != `["2",[{"id":"1","item_id":"","item_type":"","name":"","rank_type":"","time":"","gacha_type":""}]]` {
		t.Errorf("有记录时 = %s", full)
	}
}

func TestFilterPullRanks(t *testing.T) {
	d := testZZZData(t)
	recs := d.Channels[0].Records // 最新在前：B2、S1、A1、B1
	if got := filterPullRanks(recs, pullFilterAll); len(got) != 4 {
		t.Errorf("全部筛选 %d 条，期望 4 条", len(got))
	}
	sa := filterPullRanks(recs, pullFilterSA)
	if len(sa) != 2 || sa[0].Name != "S1" || sa[1].Name != "A1" {
		t.Errorf("S+A 筛选 %+v", sa)
	}
	s := filterPullRanks(recs, pullFilterS)
	if len(s) != 1 || s[0].Name != "S1" {
		t.Errorf("仅 S 筛选 %+v", s)
	}
}

func TestFormatInt(t *testing.T) {
	for n, want := range map[int]string{0: "0", 79: "79", 711: "711", 4698: "4,698", 1234567: "1,234,567"} {
		if got := formatInt(n); got != want {
			t.Errorf("formatInt(%d) = %q，期望 %q", n, got, want)
		}
	}
}
