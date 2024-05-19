package main

import (
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"

	"log"

	"github.com/dustin/go-humanize"
	pfile "github.com/qydysky/part/file"
	preqf "github.com/qydysky/part/reqf"
	pweb "github.com/qydysky/part/web"

	"github.com/signintech/gopdf"
)

func main() {
	addrp := flag.String("addr", "0.0.0.0:20000", "addr")
	flag.Parse()

	w := pweb.New(&http.Server{
		Addr: *addrp,
	})
	defer w.Shutdown()

	w.Handle(map[string]func(w http.ResponseWriter, r *http.Request){
		`/`: func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "POST" {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			if buf, err := io.ReadAll(r.Body); err != nil {
				log.Default().Println(err)
				w.WriteHeader(http.StatusBadRequest)
			} else {
				defer r.Body.Close()
				if err := deal(buf, w); err != nil {
					log.Default().Println(err)
					w.WriteHeader(http.StatusBadRequest)
				}
			}
		},
	})

	log.Default().Printf("generate pdf listen: %s\n", *addrp)
	defer log.Default().Printf("generate pdf stop\n")

	// ctrl+c退出
	var interrupt = make(chan os.Signal, 2)
	signal.Notify(interrupt, os.Interrupt)
	<-interrupt
	go func() {
		<-interrupt
		os.Exit(1)
	}()
}

var fontMap sync.Map

func deal(configBuf []byte, w io.Writer) error {
	var c Config
	if e := json.Unmarshal(configBuf, &c); e != nil {
		return errors.New("解析错误")
	}

	var loadedFontMap = make(map[string]bool)

	pdf := gopdf.GoPdf{}
	pdf.Start(gopdf.Config{PageSize: *gopdf.PageSizeA4})
	for i := 0; i < len(c.Fonts); i++ {
		if _, ok := loadedFontMap[c.Fonts[i].Family]; ok {
			continue
		}
		var buf []byte
		if b, ok := fontMap.Load(c.Fonts[i].Family); !ok {
			log.Default().Println("load font:", c.Fonts[i].Loc)
			if b, e := get(c.Fonts[i].Loc); e != nil {
				return errors.New("添加字体错误")
			} else {
				fontMap.Store(c.Fonts[i].Family, b)
				buf = b
			}
		} else {
			buf = b.([]byte)
		}
		err := pdf.AddTTFFontData(c.Fonts[i].Family, buf)
		if err != nil {
			return errors.New("添加字体错误")
		}
		loadedFontMap[c.Fonts[i].Family] = true
	}

	for i := 0; i < len(c.Page); i++ {
		p := c.Page[i]

		SetDefault(&c.Style, &p.Style)

		pdf.AddPageWithOption(gopdf.PageOption{PageSize: &gopdf.Rect{W: p.PageSizeWH[0], H: p.PageSizeWH[1]}})

		for k := 0; k < len(p.Object); k++ {
			o := p.Object[k]

			SetDefault(&p.Style, &o.Style)

			if len(c.Fonts) == 0 {
				if _, ok := loadedFontMap[o.Font.Family]; !ok {
					if b, ok := fontMap.Load(o.Font.Family); !ok {
						return errors.New("添加字体错误")
					} else {
						err := pdf.AddTTFFontData(o.Font.Family, b.([]byte))
						if err != nil {
							return errors.New("添加字体错误")
						}
						loadedFontMap[o.Font.Family] = true
					}
				}
			}

			if err := pdf.SetFont(o.Font.Family, o.Font.Style, o.Font.Size); err != nil {
				log.Default().Println(err, o.Font.Family, o.Font.Style, o.Font.Size)
				return errors.New("设置字体错误")
			}
			if len(o.Font.ColorRGB) == 3 {
				pdf.SetTextColor(o.Font.ColorRGB[0], o.Font.ColorRGB[1], o.Font.ColorRGB[2])
			}

			switch o.Type {
			case "br":
				fin := offsetF(&pdf, o.Offset)
				pdf.Br(o.Wh[1])
				fin()
			case "text":
				if len(o.Xy) == 0 {
					o.Xy = append(o.Xy, []float64{pdf.GetX(), pdf.GetY()})
				}
				for j := 0; j < len(o.Xy); j++ {
					if len(o.Xy[j]) >= 2 {
						pdf.SetXY(o.Xy[j][0], o.Xy[j][1])
					}

					fin := offsetF(&pdf, o.Offset)

					// align
					align := gopdf.Left
					switch o.Align {
					case "Center":
						align = gopdf.Center
					case "Right":
						align = gopdf.Right
					case "Top":
						align = gopdf.Top
					case "Bottom":
						align = gopdf.Bottom
					case "Middle":
						align = gopdf.Middle
					}

					switch o.Flex {
					case "x":
						if w, err := pdf.MeasureTextWidth(o.Value); err != nil {
							return errors.New("测量文本错误")
						} else {
							o.Wh[0] = w
						}
						if err := pdf.CellWithOption(&gopdf.Rect{W: o.Wh[0], H: o.Wh[1]}, o.Value, gopdf.CellOption{Align: align}); err != nil {
							return errors.New("添加文本错误")
						}
					case "y":
						op := 0
						l := 1
						for op+l <= len(o.Value) {
							if ok, _, _ := pdf.IsFitMultiCell(&gopdf.Rect{W: o.Wh[0], H: o.Wh[1]}, o.Value[op:op+l]); !ok {
								l -= 1
								if err := pdf.CellWithOption(&gopdf.Rect{W: o.Wh[0], H: o.Wh[1]}, o.Value[op:op+l], gopdf.CellOption{Align: align}); err != nil {
									return errors.New("添加文本错误")
								}
								pdf.Br(o.Wh[1])
								op += l
								l = 1
							}
							l += 1
						}
						if err := pdf.CellWithOption(&gopdf.Rect{W: o.Wh[0], H: o.Wh[1]}, o.Value[op:op+l-1], gopdf.CellOption{Align: align}); err != nil {
							return errors.New("添加文本错误")
						}
					case "content":
						for i := o.Font.Size - 1; i > 0; i-- {
							if ok, _, _ := pdf.IsFitMultiCell(&gopdf.Rect{W: o.Wh[0], H: o.Wh[1]}, o.Value); ok {
								break
							}
							if err := pdf.SetFont(o.Font.Family, o.Font.Style, i); err != nil {
								return errors.New("缩放字体错误")
							}
						}
						if err := pdf.CellWithOption(&gopdf.Rect{W: o.Wh[0], H: o.Wh[1]}, o.Value, gopdf.CellOption{Align: align}); err != nil {
							return errors.New("添加文本错误")
						}
					default:
						if err := pdf.CellWithOption(&gopdf.Rect{W: o.Wh[0], H: o.Wh[1]}, o.Value, gopdf.CellOption{Align: align}); err != nil {
							return errors.New("添加文本错误")
						}
					}

					fin()
				}
			case "line":
				pdf.SetLineWidth(o.Wh[0])
				pdf.SetLineType(o.Value)
				fx, fy := pdf.GetX(), pdf.GetY()
				for j := 0; j < len(o.Xy); j++ {
					if o.Xy[j][0] >= 0 {
						fx = o.Xy[j][0]
					}
					if o.Xy[j][1] >= 0 {
						fy = o.Xy[j][1]
					}
					if len(o.Offset) >= 2 {
						fx += o.Offset[0]
						fy += o.Offset[1]
					}
					tx, ty := o.Xy[j][2], o.Xy[j][3]
					pdf.Line(fx, fy, fx+tx, fy+ty)
					fx += tx
					fy += ty
				}
			case "xy":
				pdf.SetXY(o.Xy[0][0], o.Xy[0][1])
			default:
			}
		}
	}
	if _, e := pdf.WriteTo(w); e != nil {
		return e
	}
	return nil
}

func offsetF(pdf *gopdf.GoPdf, offset []float64) func() {
	if len(offset) >= 2 {
		pdf.SetXY(pdf.GetX()+offset[0], pdf.GetY()+offset[1])
		return func() { pdf.SetXY(pdf.GetX()-offset[0], pdf.GetY()-offset[1]) }
	}
	return func() {}
}

func get(path string) (buf []byte, err error) {
	if strings.Contains(path, "://") {
		req := preqf.New()
		err = req.Reqf(preqf.Rval{
			Url: path,
		})
		if err != nil {
			return
		}
		buf = req.Respon
	} else {
		f := pfile.New(path, 0, false)
		if !f.IsExist() {
			err = errors.New("not found")
			return
		}
		if b, e := f.ReadAll(humanize.KByte*500, humanize.MByte*100); e != nil && !errors.Is(e, io.EOF) {
			return nil, e
		} else {
			buf = b
		}
	}
	return
}
