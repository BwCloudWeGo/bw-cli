package timex

import (
	"strconv"
	"time"
)

const (
	defaultDateTimeLayout = "2006-01-02 15:04:05"
	daysPerMonth          = 30
	daysPerYear           = 365
)

// Age 使用当前时间计算生日对应的完整周岁。
func Age(birthday time.Time) int {
	return AgeAt(birthday, time.Now())
}

// AgeAt 使用指定时间计算生日对应的完整周岁。
func AgeAt(birthday time.Time, now time.Time) int {
	birthday = birthday.In(now.Location())
	if birthday.After(now) {
		return 0
	}
	age := now.Year() - birthday.Year()
	if hasNotReachedBirthdayThisYear(birthday, now) {
		age--
	}
	if age < 0 {
		return 0
	}
	return age
}

// RelativeTime 使用当前时间返回中文相对时间。
func RelativeTime(value time.Time) string {
	return RelativeTimeAt(value, time.Now())
}

// RelativeTimeAt 使用指定基准时间返回中文相对时间。
func RelativeTimeAt(value time.Time, now time.Time) string {
	return RelativeTimeInLocation(value, now, now.Location())
}

// RelativeTimeInLocation 在指定时区中返回中文相对时间。
func RelativeTimeInLocation(value time.Time, now time.Time, loc *time.Location) string {
	if loc == nil {
		loc = time.Local
	}
	value = value.In(loc)
	now = now.In(loc)
	if value.After(now) {
		value = now
	}
	diff := now.Sub(value)
	switch {
	case diff < time.Minute:
		seconds := int(diff.Seconds())
		if seconds < 1 {
			seconds = 1
		}
		return formatRelative(seconds, "秒")
	case diff < time.Hour:
		return formatRelative(int(diff.Minutes()), "分钟")
	case diff < 24*time.Hour:
		return formatRelative(int(diff.Hours()), "小时")
	case diff < daysPerMonth*24*time.Hour:
		return formatRelative(int(diff.Hours()/24), "天")
	case diff < daysPerYear*24*time.Hour:
		return formatRelative(int(diff.Hours()/(24*daysPerMonth)), "月")
	default:
		return value.Format(defaultDateTimeLayout)
	}
}

// FormatDateTime 使用 RelativeTime 超过一年后的固定格式输出时间。
func FormatDateTime(value time.Time) string {
	return value.Format(defaultDateTimeLayout)
}

func hasNotReachedBirthdayThisYear(birthday time.Time, now time.Time) bool {
	month := birthday.Month()
	day := birthday.Day()
	if month == time.February && day == 29 && !isLeapYear(now.Year()) {
		day = 28
	}
	if now.Month() != month {
		return now.Month() < month
	}
	return now.Day() < day
}

func isLeapYear(year int) bool {
	return year%400 == 0 || (year%4 == 0 && year%100 != 0)
}

func formatRelative(value int, unit string) string {
	if value < 1 {
		value = 1
	}
	return strconv.Itoa(value) + unit + "前"
}
