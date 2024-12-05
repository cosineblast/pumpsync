package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"

	"strconv"
	"strings"
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
    cmd.Stderr = os.Stderr

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
const MINIMUM_FINAL_MATCH_SCORE = 6

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

	outputFile, err := os.CreateTemp("", "pumpsync_*_ffmpeg_cut.wav")

	if err != nil {
		return "", err
	}

	outputPath := outputFile.Name()

	outputFile.Close()

	cmd := exec.Command(
		"ffmpeg",
		"-y",                           // don't ask for overwrite confirmation
		"-ss", fmt.Sprint(startOffset), // seek to this offset
		"-t", fmt.Sprint(endOffset-startOffset), // and take this many seconds
		"-i", path, // of this file
		"-af", // and remove silence from start and end
		`silenceremove=
            start_periods=1
            :start_duration=0
            :start_threshold=-50dB
        ,areverse
        ,silenceremove=
            start_periods=1
            :start_duration=0
            :start_threshold=-50dB
        ,areverse`,
		outputPath)

    cmd.Stderr = os.Stderr
	err = cmd.Run()

	if err != nil {
		return "", err
	}

	return outputPath, nil
}

func trimAndLocate(foregroundPath string, backgroundPath string) (string, float64, float64, error) {

	log.Println("Checking if foreground audio needs a cut...")

	match, err := FocusAudio(foregroundPath)

	finalForegroundPath := foregroundPath

	if err != nil {
		focusFail, ok := err.(FocusFail)
		if !ok {
			log.Println("error while trying to focus:", err)
			return "", 0, 0, err
		}

		log.Println("file did not match with known delimiters")
		log.Println("attempts:", focusFail.attempts)

		os.Exit(1)
		// TODO: trim audio from this audio nonetheless
	} else {
		log.Printf("file matched delimiter %s (%f, %f)!\n", match.Identifier, match.StartScore, match.EndScore)

		log.Printf("performing cut to range (%f:%f)\n", match.LeftCut, match.RightCut)

		result, err := CutAudio(foregroundPath, match.LeftCut, match.RightCut)

		if err != nil {
			log.Println("failed to cut audio:", err)
			return "", 0, 0, err
		}

		finalForegroundPath = result
	}

	offset, score, _, err := locateAudio(backgroundPath, finalForegroundPath)

	if err != nil {
		log.Println("faield to locate fg audio in bg:", err)
		return "", 0, 0, err
	}

	return finalForegroundPath, offset, score, nil
}

func getFileDuration(path string) (float64, error) {

	log.Printf("Getting duration of '%s'", path)

	cmd := exec.Command("ffprobe", "-i", path, "-show_entries", "format=duration", "-of", "csv=p=0")
    cmd.Stderr = os.Stderr

	stdout, err := cmd.Output()

	if err != nil {
		log.Println("failed to run ffprobe to get file duration")
		return 0, err
	}

	result, err := strconv.ParseFloat(strings.TrimSpace(string(stdout)), 64)

	if err != nil {
		return 0, err
	}

	return result, nil
}

func overwriteAudioSegment(foregroundPath string, backgroundPath string, offset float64) (string, error) {

	foregroundDuration, err := getFileDuration(foregroundPath)

	if err != nil {
		return "", err
	}

	outputFile, err := os.CreateTemp("", "pumpsync_*_ffmpeg_overwrite.wav")

	if err != nil {
		return "", err
	}

	outputPath := outputFile.Name()

	outputFile.Close()

	filterGraph := fmt.Sprintf(
		`
         [0:0]adelay=all=1:delays=%d[fg];
         [1:0]volume=volume=0:enable='between(t,%f,%f)'[bg];
         [bg][fg]amix=inputs=2:duration=longest[result]
         `,
		int(offset*1000.0),
		offset,
		offset+foregroundDuration,
	)

	cmd := exec.Command(
		"ffmpeg",
		"-y",              // don't ask for overwrite confirmation
		"-filter_complex", // use the following filter graph
		filterGraph,
		"-i", foregroundPath, // read from this file as source 0
		"-i", backgroundPath, // read from this file as source 1
		"-map", "[result]", // use cable `result` to write to output
		outputPath)
    cmd.Stderr = os.Stderr

	err = cmd.Run()

	if err != nil {
		log.Println("failed to run overwite ffmpeg")
		return "", nil
	}

	return outputPath, nil
}

func downloadYoutubeVideo(link string) (string, error) {

	outputFile, err := os.CreateTemp("", "pumpsync_*_yt_dlp.mp4")

	if err != nil {
		return "", err
	}

	outputPath := outputFile.Name()

	outputFile.Close()

    cmd := exec.Command("yt-dlp", link, "-f", "best[ext=mp4]", 
    "--force-overwrites",
    "-o", outputPath)
    cmd.Stderr = os.Stderr



	err = cmd.Run()

	if err != nil {
		return "", err
	}

	return outputPath, nil
}

func extractAudioFromVideo(videoPath string) (string, error) {

	audioFile, err := os.CreateTemp("", "pumpsync_vid_*.wav")

	if err != nil {
		return "", err
	}

	audioFile.Close()

	cmd := exec.Command("ffmpeg",
        "-y",
		"-i", videoPath,
        "-ar", "48000",
		audioFile.Name())
    cmd.Stderr = os.Stderr

	err = cmd.Run()

	if err != nil {
		return "", err
	}

	return audioFile.Name(), nil
}

var tooLowScoreError = errors.New("match score was too low to continue execution")

func overwriteVideoAudio(videoPath string, audioPath string, resultPath string) error {

	cmd := exec.Command("ffmpeg",
        "-y",
		"-i", videoPath,
		"-i", audioPath,
		"-map", "0:v",
		"-map", "1:0",
		"-f", "mp4",
		"-c", "copy",
		resultPath)

    cmd.Stderr = os.Stderr

	err := cmd.Run()

	if err != nil {
		return err
	}

	return nil
}

func improveVideoQualityFromYoutube(backgroundVideoPath string, youtubeLink string, outputFilePath string) error {

	foregroundVideoPath, err := downloadYoutubeVideo(youtubeLink)

	if err != nil {
		return err
	}

	defer os.Remove(foregroundVideoPath)

	backgroundAudioPath, err := extractAudioFromVideo(backgroundVideoPath)

	if err != nil {
		return err
	}

	foregroundAudioPath, err := extractAudioFromVideo(foregroundVideoPath)

	if err != nil {
		return err
	}

	trimmedPath, offset, score, err := trimAndLocate(foregroundAudioPath, backgroundAudioPath)

	if score < MINIMUM_FINAL_MATCH_SCORE {
		return tooLowScoreError
	}

	finalAudio, err := overwriteAudioSegment(trimmedPath, backgroundAudioPath, offset)

	overwriteVideoAudio(backgroundVideoPath, finalAudio, outputFilePath)

	return nil
}

func main() {
	link := flag.String("link", "", "The link to the youtube video")
	backgroundPath := flag.String("bg", "", "The path to the background video")
	outputpath := flag.String("o", "", "The location to write the modified background video")

	flag.Parse()

	if *link == "" || *backgroundPath == "" {
		log.Println("error: missing flags")
		// TODO: use cli arg lib
		os.Exit(1)
	}

    err := improveVideoQualityFromYoutube(*backgroundPath, *link, *outputpath)

    if err != nil {
        log.Fatal(err)
        os.Exit(1)
    }
}
