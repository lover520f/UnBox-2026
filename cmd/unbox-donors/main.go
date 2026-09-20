// Command unbox-donors 从爱发电 OpenAPI 导出离线捐助榜单。
package main

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	afdianQuerySponsorURL = "https://afdian.com/api/open/query-sponsor"
	sponsorsPerPage       = 100
	maxSponsorPages       = 10_000
)

// Donor 是写入 docs/donors.json 的捐助人数据。
type Donor struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Avatar    string  `json:"avatar"`
	Amount    float64 `json:"amount"`
	Anonymous bool    `json:"anonymous"`
}

// Leaderboard 是应用侧直接消费的榜单结构。
type Leaderboard struct {
	UpdatedAt string  `json:"updatedAt"`
	Donors    []Donor `json:"donors"`
}

type rawUser struct {
	UserID string `json:"user_id"`
	Name   string `json:"user_name"`
	Avatar string `json:"user_avatar"`
}

type rawSponsor struct {
	User         rawUser `json:"user"`
	AllSumAmount string  `json:"all_sum_amount"`
	Anonymous    bool    `json:"anonymous"`
}

// UnmarshalJSON 兼容爱发电可能返回的 anonymous / is_anonymous 字段。
// 查询接口的匿名字段名称没有在所有响应样例中统一；两种字段都缺失时按公开
// 条目处理。嵌套 user 内的 is_anonymous 也一并兼容。
func (s *rawSponsor) UnmarshalJSON(data []byte) error {
	type sponsorAlias rawSponsor
	var sponsor sponsorAlias
	if err := json.Unmarshal(data, &sponsor); err != nil {
		return err
	}

	var extra struct {
		IsAnonymous bool `json:"is_anonymous"`
		User        struct {
			IsAnonymous bool `json:"is_anonymous"`
		} `json:"user"`
	}
	if err := json.Unmarshal(data, &extra); err != nil {
		return err
	}

	*s = rawSponsor(sponsor)
	s.Anonymous = s.Anonymous || extra.IsAnonymous || extra.User.IsAnonymous
	return nil
}

type querySponsorParams struct {
	Page    int `json:"page"`
	PerPage int `json:"per_page"`
}

type querySponsorRequest struct {
	UserID string `json:"user_id"`
	Params string `json:"params"`
	TS     string `json:"ts"`
	Sign   string `json:"sign"`
}

type querySponsorData struct {
	List       []rawSponsor `json:"list"`
	TotalPage  int          `json:"total_page"`
	TotalCount int          `json:"total_count"`
}

type querySponsorResponse struct {
	EC   int              `json:"ec"`
	EM   string           `json:"em"`
	Data querySponsorData `json:"data"`
}

func main() {
	if err := run(context.Background(), os.Stdout, os.Stderr, http.DefaultClient, time.Now); err != nil {
		os.Exit(1)
	}
}

func run(ctx context.Context, stdout, stderr io.Writer, client *http.Client, now func() time.Time) error {
	userID := strings.TrimSpace(os.Getenv("AFDIAN_USER_ID"))
	token := strings.TrimSpace(os.Getenv("AFDIAN_TOKEN"))
	if userID == "" {
		return reportError(stderr, errors.New("缺少环境变量 AFDIAN_USER_ID"))
	}
	if token == "" {
		return reportError(stderr, errors.New("缺少环境变量 AFDIAN_TOKEN"))
	}

	sponsors, err := fetchAllSponsors(ctx, client, afdianQuerySponsorURL, token, userID)
	if err != nil {
		return reportError(stderr, fmt.Errorf("拉取爱发电捐助记录: %w", err))
	}

	leaderboard := Leaderboard{
		UpdatedAt: now().Format(time.RFC3339),
		Donors:    normalizeDonors(sponsors),
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(leaderboard); err != nil {
		return reportError(stderr, fmt.Errorf("输出捐助榜单: %w", err))
	}
	return nil
}

func reportError(stderr io.Writer, err error) error {
	fmt.Fprintf(stderr, "导出捐助榜单失败: %v\n", err)
	return err
}

func fetchAllSponsors(ctx context.Context, client *http.Client, endpoint, token, userID string) ([]rawSponsor, error) {
	all := make([]rawSponsor, 0)
	for page := 1; page <= maxSponsorPages; page++ {
		sponsors, totalPage, totalCount, err := fetchSponsorPage(ctx, client, endpoint, token, userID, page)
		if err != nil {
			return nil, fmt.Errorf("第 %d 页: %w", page, err)
		}
		all = append(all, sponsors...)

		if len(sponsors) == 0 {
			break
		}
		if totalPage > 0 && page >= totalPage {
			break
		}
		if totalCount > 0 && len(all) >= totalCount {
			break
		}
	}
	if len(all) == 0 && ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return all, nil
}

func fetchSponsorPage(
	ctx context.Context,
	client *http.Client,
	endpoint string,
	token string,
	userID string,
	page int,
) ([]rawSponsor, int, int, error) {
	paramsBytes, err := json.Marshal(querySponsorParams{Page: page, PerPage: sponsorsPerPage})
	if err != nil {
		return nil, 0, 0, fmt.Errorf("序列化请求参数: %w", err)
	}
	params := string(paramsBytes)
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	bodyBytes, err := json.Marshal(querySponsorRequest{
		UserID: userID,
		Params: params,
		TS:     ts,
		Sign:   signParams(token, params, ts, userID),
	})
	if err != nil {
		return nil, 0, 0, fmt.Errorf("序列化请求: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, 0, 0, fmt.Errorf("创建请求: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, 0, 0, fmt.Errorf("读取响应: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, 0, 0, fmt.Errorf("HTTP %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var result querySponsorResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, 0, 0, fmt.Errorf("解析响应 JSON: %w", err)
	}
	if result.EC != http.StatusOK {
		return nil, 0, 0, fmt.Errorf("接口错误 %d: %s", result.EC, result.EM)
	}
	return result.Data.List, result.Data.TotalPage, result.Data.TotalCount, nil
}

func normalizeDonors(sponsors []rawSponsor) []Donor {
	donors := make([]Donor, 0, len(sponsors))
	for _, sponsor := range sponsors {
		amount, err := strconv.ParseFloat(strings.TrimSpace(sponsor.AllSumAmount), 64)
		if err != nil || math.IsNaN(amount) || math.IsInf(amount, 0) {
			amount = 0
		}

		donor := Donor{
			ID:        sponsor.User.UserID,
			Name:      sponsor.User.Name,
			Avatar:    sponsor.User.Avatar,
			Amount:    amount,
			Anonymous: sponsor.Anonymous,
		}
		if donor.Anonymous {
			donor.ID = ""
			donor.Avatar = ""
		}
		donors = append(donors, donor)
	}

	sort.SliceStable(donors, func(i, j int) bool {
		if donors[i].Amount == donors[j].Amount {
			return donors[i].Name < donors[j].Name
		}
		return donors[i].Amount > donors[j].Amount
	})
	return donors
}

func signParams(token, params, ts, userID string) string {
	sum := md5.Sum([]byte(token + "params" + params + "ts" + ts + "user_id" + userID))
	return hex.EncodeToString(sum[:])
}
