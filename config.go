package main

type Config struct {
	Fonts []struct {
		Family string `json:"family"`
		Loc    string `json:"loc"`
	} `json:"fonts"`
	Style
	Page []struct {
		Style
		Object []struct {
			Style
			Type   string      `json:"type"`
			Xy     [][]float64 `json:"xy"`
			Offset []float64   `json:"offset"`
			Align  string      `json:"align"` //Left,Center,Right,Top,Bottom,Middle
			Flex   string      `json:"flex"`
			Value  string      `json:"value"`
		} `json:"object"`
	} `json:"page"`
}

type Style struct {
	PageSizeWH []float64 `json:"pageSizeWH"`
	Font       struct {
		Family   string  `json:"family"`
		Style    string  `json:"style"`
		Size     int     `json:"size"`
		ColorRGB []uint8 `json:"colorRGB"`
	} `json:"font"`
	Wh []float64 `json:"wh"`
}

func SetDefault(s, d *Style) {
	// wh
	if len(d.Wh) == 0 {
		d.Wh = s.Wh
	} else {
		for i := 0; i < len(d.Wh); i++ {
			if d.Wh[i] < 0 {
				d.Wh[i] = s.Wh[i]
			}
		}
	}

	// font
	if d.Font.Family == "" {
		d.Font.Family = s.Font.Family
	}
	if d.Font.Style == "" {
		d.Font.Style = s.Font.Style
	}
	if len(d.Font.ColorRGB) == 0 {
		d.Font.ColorRGB = s.Font.ColorRGB
	}
	if d.Font.Size == 0 {
		d.Font.Size = s.Font.Size
	}

	// pageSizeWH
	if len(d.PageSizeWH) == 0 {
		d.PageSizeWH = s.PageSizeWH
	} else {
		for i := 0; i < len(d.PageSizeWH); i++ {
			if d.PageSizeWH[i] < 0 {
				d.PageSizeWH[i] = s.PageSizeWH[i]
			}
		}
	}
}
