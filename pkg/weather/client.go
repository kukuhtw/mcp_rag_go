// pkg/weather/client.go
// Stub untuk integrasi weather API eksternal

package weather

import "fmt"

type Weather struct {
	Location string
	TempC    float64
	Condition string
}

type Client struct{}

func (c *Client) GetForecast(location string) (Weather, error) {
	// stub: return dummy data
	return Weather{
		Location: location,
		TempC:    32.0,
		Condition: "Sunny",
	}, nil
}

func (c *Client) String(w Weather) string {
	return fmt.Sprintf("%s: %.1f C, %s", w.Location, w.TempC, w.Condition)
}
