package mediasync

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

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

type audioMatch = struct {
	Offset         float64 `json:"offset"`
	Score          float64 `json:"score"`
}

func locateAudio(haystackPath string, needlePath string) (float64, float64, error) {
	log.Println("running locate script")
	cmd := newCommand("./locate_audio", haystackPath, needlePath)

	stdout, err := cmd.Output()

	if errors.Is(err, exec.ErrDot) {
		err = nil
	}

	if err != nil {
		return 0, 0, err
	}

	var message audioMatch
	message.Offset = 0
	message.Score = 0

	err = json.Unmarshal(stdout, &message)

	if err != nil {
		return 0, 0, err
	}

	return message.Offset, message.Score, nil
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

func focusAudio(path string) (*FocusSuccess, error) {

	audioPairs := []struct {
		key       string
		startPath string
		endPath   string
	}{
		{"XX", "./res/xx_start_of_music.wav", "./res/xx_end_of_music.wav"},
		{"Phoenix", "./res/phoenix_start_of_music.wav", "./res/phoenix_end_of_music.wav"},
	}

	attempts := make(map[string]FloatPair)

	for _, entry := range audioPairs {

		startOffset, startScore, err := locateAudio(path, entry.startPath)

		if err != nil {
			return nil, err
		}

        startDuration, err := getFileDuration(entry.startPath)

        if err != nil {
            return nil, err
        }

		log.Println("checking if audio matches ", entry.key)

		endOffset, endScore, err := locateAudio(path, entry.endPath)

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

func newCommand(name string, commands ...string) *exec.Cmd {
    result := exec.Command(name, commands...)

    if os.Getenv("PUMPSYNC_DEBUG") == "1" {
        result.Stderr = os.Stderr
    }

    return result
}

func trimAudioSilence(path string) (string, error) {
	outputFile, err := os.CreateTemp("", "pumpsync_*_ffmpeg_trim.wav")

	if err != nil {
		return "", err
	}

	defer func() {
		if err != nil {
			os.Remove(outputFile.Name())
		}
	}()

	outputFile.Close()

	outputPath := outputFile.Name()

	cmd := newCommand(
		"ffmpeg",
		"-y",       // don't ask for overwrite confirmation
		"-i", path, // read from this file as input 0
		"-af", // and remove silence from start and end
		`silenceremove=
            start_periods=1
            :start_duration=0
            :start_threshold=-30dB
        ,areverse
        ,silenceremove=
            start_periods=1
            :start_duration=0
            :start_threshold=-30dB
        ,areverse`,
		outputPath)

	log.Println("running ffmpeg for silence removal")
	err = cmd.Run()

	if err != nil {
		return "", err
	}

	return outputPath, nil
}

func cutAudio(path string, startOffset float64, endOffset float64) (string, error) {

	outputFile, err := os.CreateTemp("", "pumpsync_*_ffmpeg_cut.wav")

	if err != nil {
		return "", err
	}

	defer func() {
		if err != nil {
			os.Remove(outputFile.Name())
		}
	}()

	outputPath := outputFile.Name()

	outputFile.Close()

	cmd := newCommand(
		"ffmpeg",
		"-y",                           // don't ask for overwrite confirmation
		"-ss", fmt.Sprint(startOffset), // seek to this offset
		"-t", fmt.Sprint(endOffset-startOffset), // and take this many seconds
		"-i", path, // of this file
		outputPath)

	log.Println("running ffmpeg for cut")
	err = cmd.Run()

	if err != nil {
		return "", err
	}

	return outputPath, nil
}

func focusAndTrimPumpAudio(foregroundPath string) (string, error) {

	log.Println("Checking if foreground audio needs a cut...")

	match, err := focusAudio(foregroundPath)

	if err != nil {
		focusFail, ok := err.(FocusFail)
		if !ok {
			log.Println("error while trying to focus:", err)
			return "", err
		}

		log.Println("file did not match with known delimiters")
		log.Println("scores:", focusFail.attempts)

		result, err := trimAudioSilence(foregroundPath)

		if err != nil {
			return "", err
		}

		return result, nil

	} else {
		log.Printf("file matched delimiter %s (%f, %f)!\n", match.Identifier, match.StartScore, match.EndScore)

		log.Printf("performing cut to range (%f:%f)\n", match.LeftCut, match.RightCut)

		cutted, err := cutAudio(foregroundPath, match.LeftCut, match.RightCut)

		if err != nil {
			return "", err
		}

		defer func() {
			if err != nil {
				os.Remove(cutted)
			}
		}()

		trimmed, err := trimAudioSilence(cutted)

		if err != nil {
			return "", err
		}

		return trimmed, err
	}
}

func getFileDuration(path string) (float64, error) {

	log.Printf("Getting duration of '%s'", path)

	cmd := newCommand("ffprobe", "-i", path, "-show_entries", "format=duration", "-of", "csv=p=0")
	log.Println("running ffprobe")

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

	defer func() {
		if err != nil {
			os.Remove(outputFile.Name())
		}
	}()

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

	cmd := newCommand(
		"ffmpeg",
		"-y",              // don't ask for overwrite confirmation
		"-filter_complex", // use the following filter graph
		filterGraph,
		"-i", foregroundPath, // read from this file as source 0
		"-i", backgroundPath, // read from this file as source 1
		"-map", "[result]", // use cable `result` to write to output
		outputPath)

	log.Println("running ffmpeg to overwrite bg audio with fg audio")
	log.Println("offset: ", offset)
	log.Println("fgduration", foregroundDuration)

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

	defer func() {
		if err != nil {
			os.Remove(outputFile.Name())
		}
	}()

	outputFile.Close()

	outputPath := outputFile.Name()

	cmd := newCommand("yt-dlp", link, "-f", "mp4",
		"--force-overwrites",
		"--max-filesize", "512M",
		"--no-playlist",
		"-o", outputPath)

	log.Println("running yt-dlp")

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

	defer func() {
		if err != nil {
			os.Remove(audioFile.Name())
		}
	}()

	audioFile.Close()

	cmd := newCommand("ffmpeg",
		"-y",
		"-i", videoPath,
		"-ar", "44100",
        "-ac", "1",
		audioFile.Name())

	log.Println("running ffmpeg to convert video to audio")

	err = cmd.Run()

	if err != nil {
		return "", err
	}

	return audioFile.Name(), nil
}


func overwriteVideoAudio(videoPath string, audioPath string, resultPath string) error {

	cmd := newCommand("ffmpeg",
		"-y",
		"-i", videoPath,
		"-i", audioPath,
		"-map", "0:v",
		"-map", "1:0",
		"-f", "mp4",
        "-c:v", "copy",
        "-c:a", "aac",
		resultPath)

	log.Println("running ffmpeg to overwrite video audio")

	err := cmd.Run()

	if err != nil {
		return err
	}

	return nil
}

var TooLowScoreError = errors.New("audio match score was too low")

var DownloadError = errors.New("video download failed")

func ImproveAudio(backgroundVideoPath string, youtubeLink string) (string, error) {

	foregroundVideoPath, err := downloadYoutubeVideo(youtubeLink)

	if err != nil {
		return "", fmt.Errorf("[%w] %w", DownloadError, err)
	}

	defer os.Remove(foregroundVideoPath)

	backgroundAudioPath, err := extractAudioFromVideo(backgroundVideoPath)

	defer os.Remove(backgroundAudioPath)

	if err != nil {
		return "", err
	}

	foregroundAudioPath, err := extractAudioFromVideo(foregroundVideoPath)

	defer os.Remove(foregroundAudioPath)

	if err != nil {
		return "", err
	}

	trimmedForegroundAudioPath, err := focusAndTrimPumpAudio(foregroundAudioPath)

	defer os.Remove(trimmedForegroundAudioPath)

	if err != nil {
		return "", err
	}

	offset, score, err := locateAudio(backgroundAudioPath, trimmedForegroundAudioPath)

    if err != nil {
        return "", err
    }

	if score < MINIMUM_FINAL_MATCH_SCORE {
		return "", fmt.Errorf("[%w] %f", TooLowScoreError, score)
	}

	finalAudio, err := overwriteAudioSegment(trimmedForegroundAudioPath, backgroundAudioPath, offset)

    if err != nil {
        return "", err
    }

	defer os.Remove(finalAudio)

	outputFile, err := os.CreateTemp("", "pumpsync_result_*.mp4")

	if err != nil {
		return "", err
	}

	outputFilePath := outputFile.Name()

	outputFile.Close()

	defer func() {
		if err != nil {
			os.Remove(outputFilePath)
		}
	}()

	err = overwriteVideoAudio(backgroundVideoPath, finalAudio, outputFilePath)

	if err != nil {
		return "", err
	}

	return outputFilePath, nil
}
