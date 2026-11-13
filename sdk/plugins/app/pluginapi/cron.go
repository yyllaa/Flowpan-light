package pluginapi

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func ValidateCron(value string) error {
	fields := strings.Fields(strings.TrimSpace(value))
	if len(fields) != 6 {
		return fmt.Errorf("需要 6 段 cron：秒 分 时 日 月 周，例如 0 0 8 * * *")
	}
	if fields[0] != "0" {
		return fmt.Errorf("秒字段请固定为 0，当前定时任务按分钟检查")
	}
	ranges := [][2]int{{0, 59}, {0, 59}, {0, 23}, {1, 31}, {1, 12}, {0, 7}}
	for index, field := range fields {
		if err := validateCronField(field, ranges[index][0], ranges[index][1]); err != nil {
			return fmt.Errorf("第 %d 段无效：%w", index+1, err)
		}
	}
	return nil
}

func CronMatches(value string, now time.Time) bool {
	fields := strings.Fields(strings.TrimSpace(value))
	if len(fields) != 6 {
		return false
	}
	values := []int{
		now.Second(),
		now.Minute(),
		now.Hour(),
		now.Day(),
		int(now.Month()),
		int(now.Weekday()),
	}
	ranges := [][2]int{{0, 59}, {0, 59}, {0, 23}, {1, 31}, {1, 12}, {0, 7}}
	for index, field := range fields {
		if !cronFieldMatches(field, values[index], ranges[index][0], ranges[index][1]) {
			if index == 5 && values[index] == 0 && cronFieldMatches(field, 7, ranges[index][0], ranges[index][1]) {
				continue
			}
			return false
		}
	}
	return true
}

func validateCronField(field string, min int, max int) error {
	if field == "" {
		return fmt.Errorf("不能为空")
	}
	for _, part := range strings.Split(field, ",") {
		if !validateCronFieldPart(strings.TrimSpace(part), min, max) {
			return fmt.Errorf("%s 超出范围 %d-%d", field, min, max)
		}
	}
	return nil
}

func validateCronFieldPart(part string, min int, max int) bool {
	if part == "*" {
		return true
	}
	base := part
	if left, step, ok := strings.Cut(part, "/"); ok {
		base = left
		stepValue, err := strconv.Atoi(step)
		if err != nil || stepValue <= 0 {
			return false
		}
	}
	if base == "*" {
		return true
	}
	if left, right, ok := strings.Cut(base, "-"); ok {
		start, err1 := strconv.Atoi(left)
		end, err2 := strconv.Atoi(right)
		return err1 == nil && err2 == nil && start >= min && end <= max && start <= end
	}
	value, err := strconv.Atoi(base)
	return err == nil && value >= min && value <= max
}

func cronFieldMatches(field string, value int, min int, max int) bool {
	for _, part := range strings.Split(field, ",") {
		if cronFieldPartMatches(strings.TrimSpace(part), value, min, max) {
			return true
		}
	}
	return false
}

func cronFieldPartMatches(part string, value int, min int, max int) bool {
	if part == "" {
		return false
	}
	step := 1
	base := part
	if left, right, ok := strings.Cut(part, "/"); ok {
		base = left
		parsed, err := strconv.Atoi(right)
		if err != nil || parsed <= 0 {
			return false
		}
		step = parsed
	}
	start, end := min, max
	switch {
	case base == "*":
	case strings.Contains(base, "-"):
		left, right, _ := strings.Cut(base, "-")
		parsedStart, err1 := strconv.Atoi(left)
		parsedEnd, err2 := strconv.Atoi(right)
		if err1 != nil || err2 != nil {
			return false
		}
		start, end = parsedStart, parsedEnd
	default:
		parsed, err := strconv.Atoi(base)
		if err != nil {
			return false
		}
		start, end = parsed, parsed
	}
	if value < start || value > end {
		return false
	}
	return (value-start)%step == 0
}
