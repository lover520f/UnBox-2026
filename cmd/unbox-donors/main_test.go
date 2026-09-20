package main

import (
	"crypto/md5"
	"encoding/hex"
	"testing"
)

func TestNormalizeDonorsSortsByAmountAndRedactsAnonymous(t *testing.T) {
	got := normalizeDonors([]rawSponsor{
		{User: rawUser{UserID: "u2", Name: "乙", Avatar: "a2"}, AllSumAmount: "10.5"},
		{User: rawUser{UserID: "u1", Name: "甲", Avatar: "a1"}, AllSumAmount: "30"},
		{User: rawUser{UserID: "u3", Name: "丙", Avatar: "a3"}, AllSumAmount: "10.5", Anonymous: true},
	})
	if len(got) != 3 {
		t.Fatalf("归一化后条数 = %d，期望 3", len(got))
	}
	// 金额降序；同额按昵称稳定排序（丙 与 乙 同为 10.5）
	if got[0].Name != "甲" || got[1].Name != "丙" || got[2].Name != "乙" {
		t.Fatalf("排序错误: %#v", got)
	}
	// 匿名条目不得泄漏 ID 与头像
	if got[1].ID != "" || got[1].Avatar != "" {
		t.Fatalf("匿名条目未脱敏: %#v", got[1])
	}
}

func TestSignParamsMatchesAfdianScheme(t *testing.T) {
	params, ts, userID, token := `{"page":1}`, "1700000000", "u1", "tok"
	want := md5Hex(token + "params" + params + "ts" + ts + "user_id" + userID)
	if got := signParams(token, params, ts, userID); got != want {
		t.Fatalf("签名 = %s，期望 %s", got, want)
	}
}

func md5Hex(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}
