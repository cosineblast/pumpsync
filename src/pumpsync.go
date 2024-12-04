package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
)

const phoenixStartPath = "./media/phoenix_start_of_music.wav"
const phoenixEndPath = "./media/phoenix_end_of_music.wav"

type FocusSuccess struct {
	LeftCut    float64
	RightCut   float64
	Identifier string
	StartScore float64
	EndScore   float64
}

type FloatPair = struct {
	left  float64
	right float64
}

type FocusFail struct {
	attempts map[string]FloatPair
}

func (f FocusFail) Error() string {
	return "Failed to find sample in file"
}

type AudioMatch = struct {
	Offset         float64 `json:"offset"`
	Score          float64 `json:"score"`
	NeedleDuration float64 `json:"needle_duration"`
}

func locateAudio(haystackPath string, needlePath string) (float64, float64, float64, error) {
	cmd := exec.Command("./src/locate_audio.py", haystackPath, needlePath)

	stdout, err := cmd.Output()

	if errors.Is(err, exec.ErrDot) {
		err = nil
	}

	if err != nil {
		return 0, 0, 0, err
	}

	var message AudioMatch
	message.Offset = 0
	message.Score = 0
	message.NeedleDuration = 0

	err = json.Unmarshal(stdout, &message)

	if err != nil {
		return 0, 0, 0, err
	}

	return message.Offset, message.Score, message.NeedleDuration, nil
}

const AUDIO_START_MINIMUM_CONFIDENCE = 20
const AUDIO_END_MINIMUM_CONFIDENCE = 15

func adjustCutOffset(startOffset float64, startDuration float64, endOffset float64) (float64, float64) {

	// because the start-of-music audio has a fade-out, it is possible (and likely) that there
	// is a tiny amount of audio from the start-of-music audio in next to audio[start_size+start_offset]
	// so it is a good idea to move a little bit to the right

	// right now we are just doing a 0.5 second cut to the right and left
	// but there might be better ways of doing this.

	return startOffset + startDuration + 0.5, endOffset - 0.5

}

func FocusAudio(path string) (*FocusSuccess, error) {

	audioPairs := []struct {
		key       string
		startPath string
		endPath   string
	}{
		{"XX", "./media/xx_start_of_music.wav", "./media/xx_end_of_music.wav"},
		{"Phoenix", "./media/phoenix_start_of_music.wav", "./media/phoenix_end_of_music.wav"},
	}

	attempts := make(map[string]FloatPair)

	for _, entry := range audioPairs {

		startOffset, startScore, startDuration, err := locateAudio(path, entry.startPath)

		if err != nil {
			return nil, err
		}

		endOffset, endScore, _, err := locateAudio(path, entry.endPath)

		if err != nil {
			return nil, err
		}

		if startScore < AUDIO_START_MINIMUM_CONFIDENCE || endScore < AUDIO_END_MINIMUM_CONFIDENCE {
			attempts[entry.key] = FloatPair{startScore, endScore}
		} else {
			leftCut, rightCut := adjustCutOffset(startOffset, startDuration, endOffset)

			return &FocusSuccess{leftCut, rightCut, entry.key, startScore, endScore}, nil
		}
	}

	return nil, FocusFail{attempts}

}

func main() {
	path := flag.String("path", "", "The path to the wav file to cut")

	flag.Parse()

	if *path == "" {
		fmt.Println("error: missing path flag")
		os.Exit(1)
	}

	fmt.Println("Focusing background...")

	match, err := FocusAudio(*path)

	if err != nil {
		focusFail, ok := err.(FocusFail)
		if !ok {
			fmt.Println("error while trying to focus:", err)
			os.Exit(1)
		}

		fmt.Println("file did not match with known delimiters")

		fmt.Println(focusFail.attempts)
        os.Exit(0)
	}

	fmt.Println("file matched delimiter!")

	fmt.Println("identifier: ", match.Identifier)
	fmt.Println("left cut: ", match.LeftCut)
	fmt.Println("right cut: ", match.RightCut)
	fmt.Println("start score: ", match.StartScore)
	fmt.Println("end score: ", match.EndScore)
}
