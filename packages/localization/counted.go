package localization

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var countedNumber = regexp.MustCompile(`[1-9][0-9]{0,8}|0`)

func (catalog *Catalog) LocalizeCounted(tag, value string) string {
	parts := strings.Split(value, " · ")
	for index, part := range parts {
		numbers := countedNumber.FindAllStringIndex(part, 2)
		if len(numbers) != 1 {
			continue
		}
		number, err := strconv.Atoi(part[numbers[0][0]:numbers[0][1]])
		if err != nil {
			continue
		}
		format := part[:numbers[0][0]] + "%d" + part[numbers[0][1]:]
		if strings.Contains(part, "%") {
			continue
		}
		if translated := catalog.Localize(tag, format); translated != format {
			parts[index] = fmt.Sprintf(translated, number)
		}
	}
	return strings.Join(parts, " · ")
}
