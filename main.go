package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/faiface/beep"
	"github.com/faiface/beep/flac"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/wav"
	"github.com/hajimehoshi/oto"
)

func main() {
	reader := bufio.NewReader(os.Stdin)

	// Load audio file
	fmt.Print("Enter the audio file path: ")
	filePath, _ := reader.ReadString('\n')
	filePath = strings.TrimSpace(filePath)

	// Open the audio file and decode based on format
	streamer, format, err := openAudioFile(filePath)
	if err != nil {
		fmt.Println("Error loading audio file:", err)
		return
	}
	defer streamer.Close()

	// Automatically detect loopable segment
	start, end := detectLoopSegment(streamer)
	fmt.Printf("Detected loop segment from %v to %v\n", start, end)

	// Initialize audio player context
	sampleRate := int(format.SampleRate)
	context, err := oto.NewContext(sampleRate, format.NumChannels, 2, 4096)
	if err != nil {
		fmt.Println("Error initializing audio player:", err)
		return
	}
	defer context.Close()

	player := context.NewPlayer()
	defer player.Close()

	// Loop the detected segment seamlessly
	seamlessLoop(player, streamer, start, end, format)
}

// openAudioFile opens and decodes the audio file.
func openAudioFile(filePath string) (beep.StreamSeekCloser, beep.Format, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, beep.Format{}, err
	}

	var (
		streamer beep.StreamSeekCloser
		format   beep.Format
	)

	// Detect format based on file extension
	if strings.HasSuffix(filePath, ".mp3") {
		streamer, format, err = mp3.Decode(file)
	} else if strings.HasSuffix(filePath, ".wav") {
		streamer, format, err = wav.Decode(file)
	} else if strings.HasSuffix(filePath, ".flac") {
		streamer, format, err = flac.Decode(file)
	} else {
		return nil, beep.Format{}, fmt.Errorf("unsupported audio format")
	}

	if err != nil {
		file.Close()
		return nil, beep.Format{}, err
	}

	return streamer, format, nil
}

// detectLoopSegment scans the audio stream to find a loopable segment.
func detectLoopSegment(streamer beep.StreamSeekCloser) (start, end int) {
	buffer := make([][2]float64, 1024)
	var threshold float64 = 0.001 // Silence threshold
	var silenceCount int

	// Default loop start and end
	start, end = 0, 0
	silenceDuration := 44100 / 10 // Duration (in samples) to count as "silence"

	for i := 0; ; i++ {
		n, ok := streamer.Stream(buffer)
		if !ok || n == 0 {
			break
		}

		// Detect a stretch of silence
		for j := 0; j < n; j++ {
			amp := (buffer[j][0] + buffer[j][1]) / 2
			if amp < threshold {
				silenceCount++
			} else {
				// Reset silence count if sound is detected
				if silenceCount > silenceDuration {
					start = end // Update loop start to previous end
					end = i*1024 + j
				}
				silenceCount = 0
			}
		}
	}
	return start, end
}

// seamlessLoop plays a segment of the audio in a loop without pauses.
func seamlessLoop(player *oto.Player, originalStreamer beep.StreamSeekCloser, start, end int, format beep.Format) {
	for {
		// Seek to loop start
		originalStreamer.Seek(start)
		playAudioSegment(player, originalStreamer, end-start, format)
	}
}

// playAudioSegment plays a specific segment of the audio.
func playAudioSegment(player *oto.Player, streamer beep.Streamer, segmentLength int, format beep.Format) {
	buffer := make([][2]float64, 1024)
	samplesPlayed := 0

	for samplesPlayed < segmentLength {
		n, ok := streamer.Stream(buffer)
		if !ok || n == 0 {
			break
		}

		// Convert buffer data to byte format and write to player
		writeBuffer := make([]byte, n*4) // 2 channels * 2 bytes per sample
		for i := 0; i < n; i++ {
			for ch := 0; ch < 2; ch++ {
				sample := int16(buffer[i][ch] * 32767)
				writeBuffer[i*4+ch*2] = byte(sample)
				writeBuffer[i*4+ch*2+1] = byte(sample >> 8)
			}
		}
		player.Write(writeBuffer)
		samplesPlayed += n
	}
}
