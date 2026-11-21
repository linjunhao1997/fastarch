package service

import (
	"net/url"
	"regexp"
	"strings"
)

// evidence 是 evidence 类问题闭环时唯一被接受的“做了”的凭据形态。
// 设计取舍：不做链接可达性探测（离线/内网链接会把正常闭环卡死），
// 但必须能指回一个可核查的对象，避免“已修好了 / done / ok”这类空头承诺。
var (
	evidenceURLRe    = regexp.MustCompile(`https?://[^\s"'<>]+`)
	evidenceCommitRe = regexp.MustCompile(`(?i)(?:sha|commit):?[0-9a-f]{7,40}\b|\b[0-9a-f]{7,40}\b`)
	evidenceTicketRe = regexp.MustCompile(`\b[A-Z][A-Z0-9_]{1,15}-\d{1,10}\b`)
	evidencePRRe     = regexp.MustCompile(`(?i)\bPR-\d+\b|#\d+|!\d+`)
	evidenceDeployRe = regexp.MustCompile(`(?i)(?:deploy|dep):[a-z0-9._-]{4,64}|\bDEP-[A-Z0-9-]{3,64}\b`)
	evidenceDocRe    = regexp.MustCompile(`(?i)\b(?:docs/)?[a-z0-9._/-]+\.(?:md|pdf)\b`)
)

// validateEvidence 校验证据“形态”，不校验网络可达性。
// 放行任一：http(s) URL、commit SHA（7–40 hex）、工单号（KEY-123 / PR-123 / #123 / !123）、
// 部署单（deploy:xxx / dep:xxx / DEP-xxx）、仓库内文档路径（*.md / *.pdf）。
// 调用方负责先判空：空证据仍返回 3001001，格式不合法返回 3001023。
func validateEvidence(raw string) error {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ErrEvidenceRequired
	}

	for _, candidate := range evidenceURLRe.FindAllString(s, -1) {
		if u, err := url.Parse(candidate); err == nil && u.Host != "" && (u.Scheme == "http" || u.Scheme == "https") {
			return nil
		}
	}
	for _, candidate := range evidenceCommitRe.FindAllString(s, -1) {
		token := strings.ToLower(strings.TrimSpace(candidate))
		// 去掉可选前缀（sha: / commit:），大小写不敏感
		if idx := strings.LastIndex(token, ":"); idx >= 0 {
			token = token[idx+1:]
		}
		// 纯数字（如 1234567）不是 commit；至少含一个 a-f 字母。
		if strings.ContainsAny(token, "abcdef") {
			return nil
		}
	}
	if evidenceTicketRe.MatchString(s) || evidencePRRe.MatchString(s) {
		return nil
	}
	if evidenceDeployRe.MatchString(s) {
		return nil
	}
	if evidenceDocRe.MatchString(s) {
		return nil
	}
	return ErrInvalidEvidenceFormat
}
