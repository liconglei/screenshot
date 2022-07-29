package screenshot

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"testing"
)

func TestCaptureWindow(t *testing.T) {
	img, err := CaptureWindow("文件资源管理器")
	if err != nil {
		t.Error(err)
	}

	fileName := "window_shoot.png"
	file, _ := os.Create(fileName)
	defer file.Close()
	png.Encode(file, img)

	fmt.Printf("%s\n", fileName)
}

func TestCaptureRect(t *testing.T) {
	//bounds := GetDisplayBounds(0)
	bounds := image.Rect(1191, 0, 1920, 1200)
	img, err := CaptureRect(bounds)
	if err != nil {
		t.Error(err)
	}

	fileName := "window_shoot.png"
	file, _ := os.Create(fileName)
	defer file.Close()
	png.Encode(file, img)

	fmt.Printf("%s\n", fileName)
}

func BenchmarkCaptureRect(t *testing.B) {
	bounds := GetDisplayBounds(0)
	t.ResetTimer()
	for i := 0; i < t.N; i++ {
		_, err := CaptureRect(bounds)
		if err != nil {
			t.Error(err)
		}
	}
}
