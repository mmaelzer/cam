package cam

import (
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strconv"
	"testing"
)

func setup() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		boundary := "gocamera"
		w.Header().Set(
			"Content-Type",
			"multipart/x-mixed-replace; boundary="+boundary,
		)
		writer := multipart.NewWriter(w)
		writer.SetBoundary(boundary)

		closed := false

		cn := w.(http.CloseNotifier).CloseNotify()

		go func() {
			<-cn
			closed = true
		}()

		frame := []byte("not really a jpeg")
		for {
			if closed {
				writer.Close()
				return
			}

			mh := make(textproto.MIMEHeader)
			mh.Set("Content-Type", "image/jpeg")
			mh.Set("Content-Length", strconv.Itoa(len(frame)))
			pw, err := writer.CreatePart(mh)

			if err != nil {
				writer.Close()
				return
			}
			pw.Write(frame)
		}
	}))
}

func TestSubscribe(t *testing.T) {
	ts := setup()
	defer ts.Close()

	camera := Camera{
		URL: ts.URL,
	}

	sub1, err := camera.Subscribe()
	if err != nil {
		t.Fatal(err)
	}
	sub2, err := camera.Subscribe()
	if err != nil {
		t.Fatal(err)
	}

	if sub1 == sub2 {
		t.Fatal("Subscription channels should not be equal")
	}

	for i := 0; i < 4; i++ {
		select {
		case frame1 := <-sub1:
			if string(frame1.Bytes) != "not really a jpeg" {
				t.Fatal("Incorrect frame sent to subscribed channel")
			}
		case frame2 := <-sub2:
			if string(frame2.Bytes) != "not really a jpeg" {
				t.Fatal("Incorrect frame sent to subscribed channel")
			}
		}
	}

	camera.stop()
}

func TestUnsubscribe(t *testing.T) {
	ts := setup()
	defer ts.Close()

	camera := Camera{
		URL: ts.URL,
	}

	sub1, err := camera.Subscribe()
	if err != nil {
		t.Fatal(err)
	}
	sub2, err := camera.Subscribe()
	if err != nil {
		t.Fatal(err)
	}

	if ok := camera.Unsubscribe(sub1); !ok {
		t.Fatal("Unable to unsubscribe channel")
	}

	if ok := camera.Unsubscribe(sub2); !ok {
		t.Fatal("Unable to unsubscribe channel")
	}

	sub3 := make(<-chan Frame)
	if ok := camera.Unsubscribe(sub3); ok {
		t.Fatal("Should not be able to unsubscribe a non-subscribed channel")
	}

	if len(camera.listeners) > 0 {
		t.Fatal("Unsubscribe did not remove all listeners")
	}
}
