package testutil

import (
	"regexp"
)

// Replaces the klog and JSON timestamps with a static timestamp to make it
// easier to assert the logs. It also replaces the line number with 000 as it
// often changes.
//
//	From: I1018 15:12:57.953433   22183 logs.go:000] log
//	To:   I0000 00:00:00.000000   00000 logs.go:000] log
//
//	From: I1018 15:12:57.953433] log
//	To:   I0000 00:00:00.000000] log
//
//	From: {"ts":1729258473588.828,"caller":"log/log.go:000","msg":"log Print","v":0}
//	To:   {"ts":0000000000000.000,"caller":"log/log.go:000","msg":"log Print","v":0}
//
//	From: 2024/10/18 15:40:50 log Print
//	To:   0000/00/00 00:00:00 log Print
func ReplaceWithStaticTimestamps(input string) string {
	input = timestampRegexpKlog.ReplaceAllString(input, "0000 00:00:00.000000   00000")
	input = timestampRegexpKlogAlt.ReplaceAllString(input, "0000 00:00:00.000000")
	input = timestampRegexpJSON.ReplaceAllString(input, `"ts":0000000000000.000`)
	input = timestampRegexpStdLog.ReplaceAllString(input, "0000/00/00 00:00:00")
	input = fileAndLineRegexpJSON.ReplaceAllString(input, `"caller":"$1.go:000"`)
	input = fileAndLineRegexpKlog.ReplaceAllString(input, " $1.go:000")
	return input
}

var (
	timestampRegexpStdLog = regexp.MustCompile(`\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}`)

	timestampRegexpKlog    = regexp.MustCompile(`\d{4} \d{2}:\d{2}:\d{2}\.\d{6} +\d+`)
	timestampRegexpKlogAlt = regexp.MustCompile(`\d{4} \d{2}:\d{2}:\d{2}\.\d{6}`)
	fileAndLineRegexpKlog  = regexp.MustCompile(` ([^:]+).go:\d+`)

	timestampRegexpJSON   = regexp.MustCompile(`"ts":\d+\.?\d*`)
	fileAndLineRegexpJSON = regexp.MustCompile(`"caller":"([^"]+).go:\d+"`)
)
