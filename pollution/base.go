package pollution

type IPollutionStrategy interface {
	Apply(content string) (string, []string)
	GetName() string
}

type IndicatorPosition int

const (
	Beginning IndicatorPosition = 0
	Trail     IndicatorPosition = 1
	Middle    IndicatorPosition = 2
	Random    IndicatorPosition = 3

	IncrementIntStrategyName string = "incremental_inc"
	FakerStrategyName        string = "faker"
)
