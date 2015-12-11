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
	"time"
)

// A Camera is a set of configuration data for an mjpeg camera
type Camera struct {
	Name      string        // name of the camera; name will be passed along with frames
	URL       string        // url of the camera
	Username  string        // optional username for basic authentication
	Password  string        // optional password for basic authentication
	Log       bool          // should log
	body      io.ReadCloser // a reference to the http response body
	listeners []chan Frame  // slice of channels returned from the Subscribe method
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

// read will read data from the response until eof or the response
// body is closed
func (cam *Camera) read(mr *multipart.Reader) {
	defer func() {
		cam.logf("[%s] Reconnecting", cam.Name)
		go cam.start()
	}()

	for i := 0; true; i++ {
		part, err := mr.NextPart()

		// cam.logf("[%s:%d] received frame", cam.Name, i)

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
		go cam.emit(frame)
	}
}

// emit will send frames to cam listeners
func (cam *Camera) emit(frame Frame) {
	// if cam.Log {
	// 	log.Printf("[%s:%d] emit frame\n", frame.CameraName, frame.Number)
	// }
	for _, l := range cam.listeners {
		l <- frame
	}
}

// Subscribe creates a new channel that receives Frames.
// To unsubscribe, pass the returned channel to the Unsubscribe method.
func (cam *Camera) Subscribe() (<-chan Frame, error) {
	var err error
	l := make(chan Frame, 20)
	cam.listeners = append(cam.listeners, l)
	if len(cam.listeners) == 1 {
		err = cam.start()
	}
	return l, err
}

// Unsubscribe removes a channel returned from a Subscribe call
// from the list of cam listeners. Unsubscribe returns a boolean
// value of whether the channel was found and removed from the listeners.
func (cam *Camera) Unsubscribe(unsub <-chan Frame) bool {
	for i, l := range cam.listeners {
		if unsub == l {
			if len(cam.listeners) == 1 {
				cam.stop()
				cam.listeners = make([]chan Frame, 0)
			} else {
				cam.listeners = append(
					cam.listeners[:i],
					cam.listeners[i+1:]...,
				)
			}
			close(l)
			return true
		}
	}
	return false
}
