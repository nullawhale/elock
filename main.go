package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/esimov/stackblur-go"
	"github.com/gotk3/gotk3/cairo"
	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/gtk"
	"github.com/msteinert/pam"
	"image"
	"image/png"
	"log"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"os/user"
	"time"
	"unicode"
)

type Output struct {
	Name string `json:"name"`
}

type StateEvent int

const (
	Idle StateEvent = iota
	Wrong
	Success
	Clear
	Validating
	Typing
)

type State struct {
	EventChan chan StateEvent
	Event     StateEvent
	wins      []*gtk.Window
}

var (
	keySet        []uint
	keySetRune    []rune
	state         *State
	infoText      string
	randIndicator float64
)

func init() {
	log.Println("init")
	event := make(chan StateEvent)
	state = &State{EventChan: event}
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	//ticker := time.NewTicker(time.Millisecond * 10)

	log.Println("elock is running")

	gtk.Init(nil)

	//state.EventChan <- Idle

	d, _ := gdk.DisplayGetDefault()
	monitorsCount := d.GetNMonitors()

	imagePaths, err := generateLockImages()
	if err != nil {
		log.Println(err)
		return
	}
	//log.Println(monitorsCount)

	if len(imagePaths) > 0 && len(imagePaths) == monitorsCount {
		for _, imagePath := range imagePaths {
			win, err := createWindow()
			//layershell.InitForWindow(win)

			log.Println(imagePath)

			if err != nil {
				log.Println(err)
				return
			}
			//m, _ := d.GetMonitor(0)

			//layershell.SetMonitor(win, m)
			//layershell.SetKeyboardMode(win, layershell.LAYER_SHELL_KEYBOARD_MODE_ON_DEMAND)
			//layershell.SetExclusiveZone(win, 200)

			overlay, err := gtk.OverlayNew()

			//img, err := gtk.ImageNewFromFile(imagePath)
			//overlay.Add(img)

			canvas, err := gtk.DrawingAreaNew()
			overlay.AddOverlay(canvas)

			win.Add(overlay)

			//win.Fullscreen()
			win.SetDefaultSize(800, 600)
			win.ShowAll()

			canvas.Connect("draw", drawIndicator)
			win.Connect("key-press-event", keyboardHandler)

			state.wins = append(state.wins, win)

			//canvas.QueueDraw()

			//go func() {
			//	for range ticker.C {
			//		canvas.QueueDraw()
			//	}
			//}()
		}

		go observeEventState()
		gtk.Main()
	}

	if err != nil {
		log.Printf("Could not create application: %v", err)
		return
	}
}

func observeEventState() {
	for {
		select {
		case event := <-state.EventChan:
			state.Event = event
			log.Println(state.Event.String())
			for _, win := range state.wins {
				win.QueueDraw()
			}
		}
	}
}

func drawIndicator(canvas *gtk.DrawingArea, cr *cairo.Context) {
	//log.Printf("state: %d\n", state)

	msgText := ""
	width := float64(canvas.GetAllocatedWidth())
	height := float64(canvas.GetAllocatedHeight())
	radius := 100.0
	x := width / 2
	y := height / 2

	color := []float64{1, 1, 1}
	switch state.Event {
	case Success:
		color = []float64{0.33, 0.65, 0.19} // #55A630
	case Wrong:
		msgText = "Wrong"
		color = []float64{0.83, 0.05, 0.05} // #D30D0D
	case Idle:
		color = []float64{0.75, 0.75, 0.75} // #BEBEBE
	case Clear:
		msgText = "Cleared"
		color = []float64{1, 0.86, 0} // #FFDB00
	case Validating:
		msgText = "Validating..."
		color = []float64{0, 0.57, 0.92} // #0091EA
	default:
		color = []float64{0.75, 0.75, 0.75} // #BEBEBE
	}
	cr.Arc(x, y, radius, 0, 2*math.Pi)
	cr.SetSourceRGB(color[0], color[1], color[2])
	cr.SetLineWidth(8)
	cr.Stroke()

	if state.Event == Typing {
		cr.Arc(x, y, radius, randIndicator, randIndicator+math.Pi/3)
		cr.SetSourceRGB(0.37, 0.37, 0.37) // #575757
		cr.SetLineWidth(9)
		cr.Stroke()
	}

	if state.Event == Idle || state.Event == Typing || state.Event == Success {
		now := time.Now().Format("15:04")
		date := time.Now().Format("02.01.2006")

		cr.SetSourceRGB(255, 255, 255)
		cr.SetFontSize(38)
		nowExt := cr.TextExtents(now)
		cr.MoveTo(x-nowExt.Width/2, y)
		cr.ShowText(now)

		cr.SetFontSize(20)
		dateExt := cr.TextExtents(date)
		cr.MoveTo(x-dateExt.Width/2, y+dateExt.Height+dateExt.Height)
		cr.ShowText(date)
	} else {
		cr.SetSourceRGB(255, 255, 255)
		cr.SetFontSize(20)
		ext := cr.TextExtents(msgText)
		cr.MoveTo(x-ext.Width/2, y+ext.Height/2)
		cr.ShowText(msgText)
	}

	if infoText != "" {
		cr.SetSourceRGB(255, 255, 255)
		cr.SetFontSize(20)
		ext := cr.TextExtents(infoText)
		cr.MoveTo(x-ext.Width/2, height*0.875-ext.Height/2)
		cr.ShowText(infoText)
	}
}

func keyboardHandler(win *gtk.Window, ev *gdk.Event) {
	keyEvent := &gdk.EventKey{Event: ev}
	keyVal := keyEvent.KeyVal()

	unicodeChar := gdk.KeyvalToUnicode(keyVal)
	if unicodeChar != 0 && !unicode.IsControl(unicodeChar) {
		randIndicator = float64((rand.Int()%360)+1) * math.Pi / 180

		keySet = append(keySet, keyVal)
		keySetRune = append(keySetRune, unicodeChar)
		state.EventChan <- Typing
		//win.QueueDraw()
		setTimeout(func() {
			state.EventChan <- Idle
			//win.QueueDraw()
		}, 500)
	}

	switch keyVal {
	case gdk.KEY_KP_Enter, gdk.KEY_Return:
		state.EventChan <- Validating
		//win.QueueDraw()
		log.Println("Checking password")
		pw := string(keySetRune)
		u, err := user.Current()
		if err != nil {
			log.Printf("%v\n", err)
		}

		_, err = submitPass(u.Username, pw)
		if err != nil {
			state.EventChan <- Wrong
			//win.QueueDraw()
			setTimeout(func() {
				infoText = ""
				state.EventChan <- Idle
				//win.QueueDraw()
			}, 1000)
			log.Printf("%v\n", err)
		} else {
			state.EventChan <- Success
			//win.QueueDraw()
			win.Close()
		}

		keySet = []uint{}
		keySetRune = []rune{}
	case gdk.KEY_Delete, gdk.KEY_BackSpace:
		if len(keySet) > 0 {
			keySet = keySet[:len(keySet)-1]
		} else if len(keySet) == 0 {
			state.EventChan <- Clear
			//win.QueueDraw()
			setTimeout(func() {
				state.EventChan <- Idle
				//win.QueueDraw()
			}, 2000)
		}
	case gdk.KEY_q:
		log.Println("q")
		win.Close()
	case gdk.KEY_u:
		eventState := keyEvent.State()
		if eventState&gdk.CONTROL_MASK == gdk.CONTROL_MASK {
			keySet = []uint{}
			keySetRune = []rune{}
			state.EventChan <- Clear
			setTimeout(func() {
				state.EventChan <- Idle
			}, 2000)
		}
	}

	//log.Printf("%v\n", keySet)
}

func setTimeout(f func(), milliseconds int) {
	wait := time.Millisecond * time.Duration(milliseconds)
	time.AfterFunc(wait, f)
}

func submitPass(uname, pw string) (string, error) {
	t, err := pam.StartFunc("elock", uname, func(s pam.Style, msg string) (string, error) {
		switch s {
		case pam.PromptEchoOff:
			return pw, nil
		case pam.PromptEchoOn:
			return "", nil
		case pam.TextInfo:
			infoText = msg
			return "", nil
		}
		return "", errors.New("unrecognized PAM message style")
	})

	if err != nil {
		return "authentication failed", err
	}

	if err = t.Authenticate(0); err != nil {
		return "authentication failed", err
	}

	infoText = ""
	return "authentication succeeded", nil
}

func createWindow() (win *gtk.Window, err error) {
	win, err = gtk.WindowNew(gtk.WINDOW_TOPLEVEL)
	if err != nil {
		return nil, fmt.Errorf("error while creating window: %v", err)
	}
	//title := fmt.Sprintf("elock %s", output)
	//win.SetTitle(title)
	win.SetResizable(false)
	win.Connect("destroy", func() {
		gtk.MainQuit()
	})
	return win, nil
}

func generateLockImages() ([]string, error) {
	var err error
	var lockArgs []string
	outputs, err := getOutputs()
	if err != nil {
		log.Printf("Error getting outputs: %v", err)
		return nil, err
	}
	log.Printf("%v\n", outputs)
	for _, output := range outputs {
		fileName := fmt.Sprintf("/tmp/%s-lock.png", output)
		grimCmd := exec.Command("grim", "-o", output, fileName)
		if err = grimCmd.Run(); err != nil {
			log.Printf("Error with grim: %v", err)
			return nil, err
		}

		img, err := os.Open(fileName)
		if err != nil {
			log.Printf("Error while opening image: %v", err)
			return nil, err
		}
		defer img.Close()

		imgSrc, _, err := image.Decode(img)
		if err != nil {
			log.Printf("Error decode image: %v", err)
			return nil, err
		}

		blurImage, err := generateBlurImage(imgSrc, fileName)
		if err != nil {
			log.Println(err)
			return nil, err
		}
		log.Printf("Name: %s\n", blurImage.Name())

		//lockArgs = append(lockArgs, fmt.Sprintf("--image=%s:%s", output, blurImage.Name()))
		lockArgs = append(lockArgs, blurImage.Name())
	}
	return lockArgs, err
}

func getOutputs() ([]string, error) {
	var outputsByte []byte
	o := exec.Command("swaymsg", "--t=get_outputs", "--raw")
	outputsByte, err := o.Output()
	if err != nil {
		return nil, err
	}
	_ = o.Run()

	var outputs []*Output
	if err = json.Unmarshal(outputsByte, &outputs); err != nil {
		return nil, err
	}

	var res []string
	for _, output := range outputs {
		res = append(res, output.Name)
	}
	if len(res) > 0 {
		return res, err
	}
	return nil, err
}

func generateBlurImage(src image.Image, name string) (*os.File, error) {
	img, err := stackblur.Process(src, 50)
	if err != nil {
		log.Fatal(err)
	}
	output, err := os.Create(name)
	if err != nil {
		return nil, err
	}
	defer output.Close()

	if err = png.Encode(output, img); err != nil {
		return nil, err
	}
	return output, err
}

func (w StateEvent) String() string {
	switch w {
	case 0:
		return "Idle"
	case 1:
		return "Wrong"
	case 2:
		return "Success"
	case 3:
		return "Clear"
	case 4:
		return "Validating"
	case 5:
		return "Typing"
	}

	return "NOT Defined"
}
