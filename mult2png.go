package main

import (
    "fmt"
    "os"
    "encoding/binary"
    "compress/zlib"
    "bytes"
    "io"
    "image/png"
    "image"
    "image/draw"
    //"image/color"
    "math"
    "strconv"
)

type Frame struct {
    Subframes []SubFrame
}

type SubFrame struct {
    Name string
    Type uint8
    X, Y float32
    W, H uint32
    Im *image.RGBA
}

func read_int32(f *os.File)(uint32) {
	var val uint32
	err := binary.Read(f, binary.LittleEndian, &val)
	if err != nil {
        panic(err)
	}
	return val
}

func read_float32(f *os.File)(float32) {
	var val float32
	err := binary.Read(f, binary.LittleEndian, &val)
	if err != nil {
        panic(err)
	}
	return val
}

func fix_palette(palette *[]byte) {

    // bug fix when overlaying a picture
    
    for i := 0; i < len(*palette); i += 4 {
        if (*palette)[i+3] == 0 {
            (*palette)[i+1] = 0
            (*palette)[i+2] = 0
            (*palette)[i+3] = 0
        }
    }
}

func save_frames(path string, frames []Frame) {

    for i, frame := range frames {  
    
        for j, sbframe := range frame.Subframes {
        
            if sbframe.Type == 0xf0 {
                filename := fmt.Sprintf("%s.%04d.%04d.png", path[:len(path)-5], i, j)
                f, err := os.Create(filename)
                if err != nil {
                     fmt.Println(err)
                     os.Exit(1)
                }
                
                png.Encode(f, sbframe.Im)

                if err := png.Encode(f, sbframe.Im); err != nil {
                     fmt.Println(err)
                     os.Exit(1)
                }
                
                f.Close()
            
            }
        }
    }
}


func save_frames_on_canvas(path string, frames []Frame, wc int, hc int) {

    for i, frame := range frames {
    
        canvas := image.NewRGBA(image.Rect(0, 0, wc, hc))
        //blue := color.RGBA{0, 125, 255, 255}
        //draw.Draw(canvas, canvas.Bounds(), &image.Uniform{blue}, image.ZP, draw.Over)
        
        if len(frame.Subframes) == 0 {
            continue
        }
    
        for _, sbframe := range frame.Subframes {
        
            im := sbframe.Im
            
            if sbframe.Type == 0xf3 {
                im = frames[sbframe.W-1].Subframes[sbframe.H].Im
            }
            
            w := im.Bounds().Size().X
            h := im.Bounds().Size().Y
            
            val_w := float64(float32(wc/2) + sbframe.X)
            val_h := float64(float32(hc/2) - sbframe.Y)
            val1_w := math.Floor(float64(w)/2)
            val1_h := math.Ceil(float64(h)/2)
            x := int(math.Floor(val_w - val1_w + 0.5))
            y := int(math.Floor(val_h - val1_h + 0.5))
            
            offset := image.Pt(x, y)
            draw.Draw(canvas, im.Bounds().Add(offset), im, image.ZP, draw.Over)


        }
        
        filename := fmt.Sprintf("%s.%04d.png", path[:len(path)-5], i)
        f, err := os.Create(filename)

        if err != nil {
             fmt.Println(err)
             os.Exit(1)
        }

        if err := png.Encode(f, canvas); err != nil {
             fmt.Println(err)
             os.Exit(1)
        }
        
        f.Close()
    }
}


func unpack_mult(f *os.File, frames *[]Frame) {

    frames_num := read_int32(f)
    colors_num := read_int32(f)
    
    palette := make([]byte, colors_num * 4)
    f.Read(palette)
    fix_palette(&palette)
    
    fmt.Println("Frames: ", frames_num)
    
    f.Seek(0x00004000, 0)
    
    for i := 0; i < int(frames_num); i++ {
        size_frame := read_int32(f)
        subframes_num := read_int32(f)
        
        if size_frame <= 4 {
            continue
        }
        
        var frame Frame
        
        for j := 0; j < int(subframes_num); j++ {
        
            var subframe SubFrame
        
            f.Seek(48, 1)
            subframe.Type = uint8(read_int32(f) & 0xff)
            size_zfile := read_int32(f)
            subframe.W = read_int32(f)
            subframe.H = read_int32(f)
            size_zfile -= 72
            
            if subframe.Type == 0xf0 {
            
                if size_zfile <= 0 {
                    f.Seek(8, 1) // coords
                    continue
                }
            
                zlib_data := make([]byte, size_zfile)
                f.Read(zlib_data)
                
                subframe.X = read_float32(f)
                subframe.Y = read_float32(f)
                
                
                b := bytes.NewReader(zlib_data)
                r, err := zlib.NewReader(b)
                if err != nil {
                    panic(err)
                }
                var out bytes.Buffer
                io.Copy(&out, r)
                
                zfilex := out.Bytes()
                
                
                num_r := 0
                rgb := make([]byte, subframe.W * subframe.H * 4)
                
                for zfile_pos := 0; zfile_pos < len(zfilex); zfile_pos += 2 {
                    unit := (uint16(zfilex[zfile_pos+1]) << 8) | uint16(zfilex[zfile_pos])
                    num_pixels := int(unit >> 12) + 1
                    color_id := (unit & 0b0000111111111111) * 4
                    
                    for k := 0; k < num_pixels; k++ {
                        rgb[num_r] = palette[color_id + 2]
                        rgb[num_r+1] = palette[color_id + 1]
                        rgb[num_r+2] = palette[color_id]
                        rgb[num_r+3] = palette[color_id + 3]
                        
                        num_r += 4;
                    }
                    
                }   
                    
                img := image.NewRGBA(image.Rect(0, 0, int(subframe.W), int(subframe.H)))
                img.Pix = rgb
                
                subframe.Im = img
                
            
            } else if subframe.Type == 0xf3 {
                subframe.X = read_float32(f)
                subframe.Y = read_float32(f)
            } else {
                fmt.Println("Unknow type %#x", subframe.Type)
                os.Exit(1)
            }
            
            frame.Subframes = append(frame.Subframes, subframe)
        }
        *frames = append(*frames, frame)
    }
}


func main() {

    args := os.Args[1:]
    w := 0
    h := 0
    
    if len(args) == 0 {
        return
    }
    
    if len(args) == 4 && args[1] == "-c" {
        w, _ = strconv.Atoi(args[2])
        h, _ = strconv.Atoi(args[3])
        if w == 0 || h == 0 {
            fmt.Println("Width and height are incorrect")
            return
        }
    }

    f, err := os.Open(args[0])
    if err != nil {
        fmt.Println(err)
        return
    }
    
    var frames []Frame
    unpack_mult(f, &frames)
    if w > 0 {
        save_frames_on_canvas(args[0], frames, w, h)
    } else {
        save_frames(args[0], frames)
    }
    
    f.Close()


}