package streaming

import (
	"bytes"
	"context"
	"image/jpeg"
	"image/png"
	"log"
	"time"

	"github.com/emulador/gateway/adb"
)

func captureLoop(ctx context.Context, adbClient *adb.ADBClient, adbPort int, containerName string, broadcast func([]byte)) {
	log.Printf("capture loop started for container %s (port %d)", containerName, adbPort)

	// Give the display a moment to initialize
	time.Sleep(3 * time.Second)

	frameCount := 0

	for {
		select {
		case <-ctx.Done():
			log.Printf("capture loop stopped for %s after %d frames", containerName, frameCount)
			return
		default:
		}

		start := time.Now()

		// Use docker exec instead of ADB to bypass network issues
		pngData, err := adbClient.ScreencapViaDocker(containerName)
		if err != nil {
			if frameCount == 0 {
				log.Printf("screencap error on %s (frame %d): %v", containerName, frameCount, err)
			}
			time.Sleep(1 * time.Second)
			continue
		}

		img, err := png.Decode(bytes.NewReader(pngData))
		if err != nil {
			log.Printf("png decode error on %s: %v (data size: %d)", containerName, err, len(pngData))
			time.Sleep(1 * time.Second)
			continue
		}

		var buf bytes.Buffer
		err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 30})
		if err != nil {
			log.Printf("jpeg encode error on %s: %v", containerName, err)
			time.Sleep(1 * time.Second)
			continue
		}

		frame := make([]byte, buf.Len())
		copy(frame, buf.Bytes())

		broadcast(frame)
		frameCount++

		if frameCount == 1 || frameCount%100 == 0 {
			log.Printf("frame %d for %s: png=%d bytes, jpeg=%d bytes, took %v", frameCount, containerName, len(pngData), len(frame), time.Since(start))
		}

		// Target ~3fps
		elapsed := time.Since(start)
		if elapsed < 333*time.Millisecond {
			time.Sleep(333*time.Millisecond - elapsed)
		}
	}
}
