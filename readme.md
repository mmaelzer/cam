cam
===

Cam is a package for connecting to and streaming data from mjpeg-cameras in Go.

[![build status](https://secure.travis-ci.org/mmaelzer/cam.png)](http://travis-ci.org/mmaelzer/cam)
[![Coverage Status](https://coveralls.io/repos/mmaelzer/cam/badge.svg?branch=master&service=github)](https://coveralls.io/github/mmaelzer/cam?branch=master)
[![godoc reference](https://godoc.org/github.com/mmaelzer/cam?status.png)](https://godoc.org/github.com/mmaelzer/cam)

## Example

```go
import (
  "fmt"
  "ioutil"

  "github.com/mmaelzer/cam"
)

func main() {
  camera := cam.Camera{
    URL: "http://64.122.208.241:8000/axis-cgi/mjpg/video.cgi?resolution=320x240"
  }

  frames, err := camera.Subscribe()
  if err != nil {
    panic(err)
  }

  for frame := range frames {
    filename := fmt.Sprintf("%d.jpeg", frame.Timestamp.UnixNano())
    ioutil.WriteFile(filename, frame.Bytes, 0644)
  }
}
```

## Usage

#### type Camera

```go
type Camera struct {
	Name     string // name of the camera; name will be passed along with frames
	URL      string // url of the camera
	Username string // optional username for basic authentication
	Password string // optional password for basic authentication
}
```

A Camera is a set of configuration data for an mjpeg camera

#### func (*Camera) Subscribe

```go
func (cam *Camera) Subscribe() (<-chan Frame, error)
```
Subscribe creates a new channel that receives Frames. To unsubscribe, pass the
returned channel to the Unsubscribe method.

#### func (*Camera) Unsubscribe

```go
func (cam *Camera) Unsubscribe(unsub <-chan Frame) bool
```
Unsubscribe removes a channel returned from a Subscribe call from the list of
cam listeners. Unsubscribe returns a boolean value of whether the channel was
found and removed from the listeners.

#### type Frame

```go
type Frame struct {
	CameraName string    // the source of frame
	Number     uint64    // a monotomically incremented frame count
	Timestamp  time.Time // time the frame was received
	Bytes      []byte    // jpeg data
}
```

A Frame is a container for jpeg data from a Camera


The MIT License
===============

Copyright (c) 2015 Michael Maelzer

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
