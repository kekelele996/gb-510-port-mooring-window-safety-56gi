package model

import "time"

// WeatherWindow models 风浪窗口 as an independently versioned aggregate. The fields
// cover ownership, operational context, evidence and measured risk so later
// changes naturally span persistence, service and UI layers.
type WeatherWindow struct {
	BaseModel
	Facility    string    `json:"facility" gorm:"size:120;index"`
	Owner       string    `json:"owner" gorm:"size:120;index"`
	Category    string    `json:"category" gorm:"size:80;index"`
	RiskLevel   string    `json:"riskLevel" gorm:"size:32;index"`
	MetricValue float64   `json:"metricValue"`
	MetricUnit  string    `json:"metricUnit" gorm:"size:24"`
	EffectiveAt time.Time `json:"effectiveAt"`
	ExpireAt    time.Time `json:"expireAt"`
	Evidence    string    `json:"evidence" gorm:"size:2000"`
	RelatedCode string    `json:"relatedCode" gorm:"size:64;index"`
}

// UsableForClearance reports whether the window may currently back a safety
// clearance submission or release. A window is only valid while explicitly
// safe and still inside its effective time range.
func (item WeatherWindow) UsableForClearance(now time.Time) bool {
	if item.Status != "safe" {
		return false
	}
	return !item.ExpireAt.Before(now) && !now.Before(item.EffectiveAt)
}

func (item *WeatherWindow) GetBase() *BaseModel { return &item.BaseModel }

func (item WeatherWindow) TableName() string { return "weather_windows" }

var WeatherWindowInitialStatus = "forecast"
