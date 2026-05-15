package matcher

import (
	"regexp"

	"mygrep/internal/domain"
)

type regexPattern struct {
	re  *regexp.Regexp
	src string
}

func (p *regexPattern) Matches(line string) bool { return p.re.MatchString(line) }
func (p *regexPattern) String() string           { return p.src }

func NewRegex(s string) (domain.Pattern, error) {
	re, err := regexp.Compile(s)
	if err != nil {
		return nil, err
	}
	return &regexPattern{re: re, src: s}, nil
}
