package pollution

import (
	"fmt"
	"github.com/melardev/discord-message-protect/utils"
	"math/rand"
	"strings"
)

type RandomStringStrategy struct {
	Position IndicatorPosition
	MinWords int
	MaxWords int

	MinWordLen int
	MaxWordLen int
}

func (f *RandomStringStrategy) GetName() string {
	return FakerStrategyName
}

type CreateRandomStringStrategyDto struct {
	Position IndicatorPosition
	MinWords int
	MaxWords int
}

func newRandomStringStrategy(dto *CreateRandomStringStrategyDto) *RandomStringStrategy {
	return &RandomStringStrategy{
		Position: dto.Position,
		MinWords: dto.MinWords,
		MaxWords: dto.MaxWords,
	}
}

func (f *RandomStringStrategy) Apply(content string) (string, []string) {
	indicators := f.GetIndicators()

	if f.Position == Beginning {
		return fmt.Sprintf("%s %s", strings.Join(indicators, " "), content), indicators
	} else if f.Position == Middle {
		wordCount := len(strings.Split(content, " "))
		halfCount := wordCount / 2
		return fmt.Sprintf("%s %s %s", content[:halfCount], strings.Join(indicators, " "), content[halfCount:]), indicators
	} else {
		return fmt.Sprintf("%s %s", content, strings.Join(indicators, " ")), indicators
	}
}

func (f *RandomStringStrategy) GetIndicators() []string {
	var indicators []string

	if f.MinWords <= 0 && f.MaxWords <= 0 {
		return []string{}
	}

	wordCount := rand.Intn(f.MaxWords-f.MinWords) + f.MinWords

	for i := 0; i < wordCount; i++ {
		strLen := rand.Intn(f.MaxWordLen-f.MinWordLen) + f.MinWordLen
		indicators = append(indicators, utils.GetRandomString(strLen))
	}

	return indicators
}
