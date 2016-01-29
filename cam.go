// Package cam provides a mjpeg camera client that allows for
// pipelining streams of jpeg data
package cam

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
	"sync"
	"time"
)

// A Camera is a set of configuration data for an mjpeg camera
type Camera struct {
	Name      string // name of the camera; name will be passed along with frames
	URL       string // url of the camera
	Username  string // optional username for basic authentication
	Password  string // optional password for basic authentication
	Log       bool   // should log
	LastFrame *Frame
	Reconnect bool          // should automatically retry
	body      io.ReadCloser // a reference to the http response body
	listeners []chan Frame  // slice of channels returned from the Subscribe method
	mutex     sync.Mutex
	locked    bool // lock to prevent multiple keepalive goroutines
}

// A Frame is a container for jpeg data from a Camera
type Frame struct {
	CameraName string    // the source of frame
	Number     uint64    // a monotomically incremented frame count
	Timestamp  time.Time // time the frame was received
	Bytes      []byte    // jpeg data
}

// start connects to the camera, parses the header information of
// the response to validate, and spawns a goroutine to read from the
// connection
func (cam *Camera) start() error {
	resp, err := cam.connect()
	if err != nil {
		return err
	}
	cam.logf("[%s] connected %s", cam.Name, cam.URL)
	cam.body = resp.Body

	ct := resp.Header.Get("Content-Type")
	mediaType, params, err := mime.ParseMediaType(ct)
	if err != nil {
		return err
	}

	boundary, ok := params["boundary"]
	if ok && strings.HasPrefix(mediaType, "multipart/") {
		reader := multipart.NewReader(resp.Body, boundary)
		cam.logf("[%s] begin reading", cam.Name)
		go cam.read(reader)
	} else {
		return fmt.Errorf("Received a non-multipart mime type from %s", cam.URL)
	}
	return nil
}

// connect handles the basic http connection to the camera
func (cam *Camera) connect() (*http.Response, error) {
	req, err := http.NewRequest("GET", cam.URL, nil)
	if err != nil {
		return nil, err
	}

	if cam.Username != "" {
		req.SetBasicAuth(cam.Username, cam.Password)
	}

	client := &http.Client{}
	return client.Do(req)
}

// stop handles signaling the connection close
func (cam *Camera) stop() {
	cam.body.Close()
}

func (cam *Camera) log(l ...interface{}) {
	if cam.Log {
		log.Print(l...)
	}
}

func (cam *Camera) logf(t string, l ...interface{}) {
	if cam.Log {
		log.Printf(t, l...)
	}
}

func (cam *Camera) keepalive() {
	if cam.locked || !cam.Reconnect {
		return
	}
	cam.locked = true
	time.Sleep(time.Second * 10)
	cam.locked = false
	if cam.LastFrame != nil &&
		time.Since(cam.LastFrame.Timestamp) > time.Second*10 {
		cam.stop()
	} else {
		go cam.keepalive()
	}
}

// read will read data from the response until eof or the response
// body is closed
func (cam *Camera) read(mr *multipart.Reader) {
	defer func() {
		if !cam.Reconnect {
			return
		}

		cam.logf("[%s] Reconnecting", cam.Name)
		err := cam.start()
		for err != nil {
			log.Printf("[%s] Unable to reconnect. Retrying...", cam.Name)
			time.Sleep(time.Second * 3)
			err = cam.start()
		}
	}()

	start := time.Now()
	frames := 0

	if cam.Reconnect {
		go cam.keepalive()
	}

	for i := 0; ; i++ {
		part, err := mr.NextPart()

		if cam.Log {
			frames++
		}

		if cam.Log && time.Since(start) > time.Minute {
			cam.logf("[%s] received %d frames in %s", cam.Name, frames, time.Since(start))
			start = time.Now()
			frames = 0
		}

		if err != nil {
			if err == io.EOF ||
				strings.Contains(err.Error(), "NextPart") {
				cam.log("EOF found")
				cam.stop()
			} else {
				cam.log(err)
			}
			return
		}

		jpeg, err := ioutil.ReadAll(part)

		if err != nil && strings.Contains(err.Error(), "Part Read") {
			return
		}

		if err != nil {
			cam.log(err)
			return
		}

		frame := Frame{
			CameraName: cam.Name,
			Number:     uint64(i),
			Bytes:      jpeg,
			Timestamp:  time.Now(),
		}
		cam.LastFrame = &frame
		cam.emit(frame)
	}
}

// emit will send frames to cam listeners
func (cam *Camera) emit(frame Frame) {
	// Since there's no way to test if a channel is closed
	// just recover
	defer func() { recover() }()
	for _, l := range cam.listeners {
		l <- frame
	}
}

// Subscribe creates a new channel that receives Frames.
// To unsubscribe, pass the returned channel to the Unsubscribe method.
func (cam *Camera) Subscribe() (<-chan Frame, error) {
	var err error
	l := make(chan Frame, 20)
	go func() {
		cam.mutex.Lock()
		if len(cam.listeners) == 0 {
			err = cam.start()
		}
		cam.listeners = append(cam.listeners, l)
		cam.mutex.Unlock()
	}()
	return l, err
}

// Unsubscribe removes a channel returned from a Subscribe call
// from the list of cam listeners. Unsubscribe returns a boolean
// value of whether the channel was found and removed from the listeners.
func (cam *Camera) Unsubscribe(unsub <-chan Frame) bool {
	for i, l := range cam.listeners {
		if unsub == l {
			go func() {
				cam.mutex.Lock()
				if len(cam.listeners) == 1 {
					cam.Stop()
				} else {
					cam.listeners = append(
						cam.listeners[:i],
						cam.listeners[i+1:]...,
					)
				}
				close(l)
				cam.mutex.Unlock()
			}()
			return true
		}
	}
	return false
}

func (cam *Camera) Stop() {
	cam.Reconnect = false
	cam.stop()
	cam.listeners = make([]chan Frame, 0)
}
