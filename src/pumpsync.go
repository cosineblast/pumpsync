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

func CutAudio(path string, startOffset float64, endOffset float64) (string, error) {

	tempFile, err := os.CreateTemp("", "pumpsync_*_ffmpeg.wav")

	if err != nil {
		return "", err
	}

	tempPath := tempFile.Name()

	tempFile.Close()

	defer func() {
		if err != nil {
			os.Remove(tempPath)
		}
	}()

	cmd := exec.Command(
        "ffmpeg", 
        "-y", // don't ask for overwrite confirmation
        "-ss", fmt.Sprint(startOffset), // seek to this offset
        "-t", fmt.Sprint(endOffset-startOffset), // and take this many seconds
        "-i", path, // of this file
		"-af", // and remove silence from start and end
		"silenceremove=start_periods=1:start_duration=0:start_threshold=-50dB:stop_periods=1:stop_duration=0:stop_threshold=-50dB",
		tempPath)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()

	if err != nil {
		return "", err
	}

	return tempPath, nil
}

func main() {
	foregroundPath := flag.String("fg", "", "The path to the foreground audio file")
	backgroundPath := flag.String("bg", "", "The path to the background audio file")

	flag.Parse()

	if *foregroundPath == "" || *backgroundPath == "" {
		fmt.Println("error: missing flags")
		fmt.Println("TODO: use cli arg lib")
		os.Exit(1)
	}

	fmt.Println("Checking if foreground audio needs a cut...")

	match, err := FocusAudio(*foregroundPath)

	finalForegroundPath := *foregroundPath

	if err != nil {
		focusFail, ok := err.(FocusFail)
		if !ok {
			fmt.Println("error while trying to focus:", err)
			os.Exit(1)
		}

		fmt.Println("file did not match with known delimiters")
		fmt.Println("attempts:", focusFail.attempts)
	} else {
		fmt.Printf("file matched delimiter %s (%f, %f)!\n", match.Identifier, match.StartScore, match.EndScore)

		fmt.Printf("performing cut to range (%f:%f)\n", match.LeftCut, match.RightCut)

		result, err := CutAudio(*foregroundPath, match.LeftCut, match.RightCut)

		if err != nil {
			fmt.Println("failed to cut audio:", err)
			os.Exit(1)
		}

		finalForegroundPath = result

		fmt.Println("result path: ", result)
	}

	offset, score, _, err := locateAudio(*backgroundPath, finalForegroundPath)

	if err != nil {
		fmt.Println("faield to locate fg audio in bg:", err)
		os.Exit(1)
	}

	fmt.Println("offset: ", offset)
	fmt.Println("score: ", score)
}
