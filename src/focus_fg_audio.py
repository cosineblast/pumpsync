
import numpy as np
import numpy.random
import numpy.fft

import matplotlib.pyplot as plt
import pandas as pd
import scipy.io.wavfile as wavfile
import scipy.signal

import sys

import fire

from multiprocessing import Pool

SUPPORTED_SAMPLE_RATE = 48_000



def _get_xx_start_audio():
    return _get_audio('./media/xx_start_of_music.wav')

def _get_xx_end_audio():
    return _get_audio('./media/xx_end_of_music.wav')

def _get_phoenix_start_audio():
    return _get_audio('./media/phoenix_start_of_music.wav')

def _get_phoenix_end_audio():
    return _get_audio('./media/phoenix_end_of_music.wav')

def _get_audio(path):
    sample_rate, data = scipy.io.wavfile.read(path)

    assert sample_rate == SUPPORTED_SAMPLE_RATE

    return _make_stereo(data).astype('float')

def _make_stereo(data):
    if len(data) == 2 and data.shape[1] == 2:
        transposed = np.transpose(data)
        return (transposed[0] + transposed[1]) / 2.0

    return np.copy(data)



def _locate_sample(haystack, needle):
    """
    Returns (offset, confidence)
    """

    correlation = scipy.signal.correlate(haystack, needle)

    audio_start = int(np.argmax(correlation)) - len(needle) + 1

    z_score = (np.max(correlation)- np.mean(correlation)) / np.std(correlation)

    return (audio_start, float(z_score))


AUDIO_START_MINIMUM_CONFIDENCE = 20
AUDIO_END_MINIMUM_CONFIDENCE = 15

_audio_sample_files = {
    'xx': (_get_xx_start_audio, _get_xx_end_audio),
    'phoenix': (_get_phoenix_start_audio, _get_phoenix_end_audio)
}


def focus_fg_audio(audio):
    """
    Returns
    (False, match_confidence_dict) ->
    I could not locate any relevant delimiters with high confidence.

    (True, left_cut, right_cut, identifier, start_confidence, end_confidence) ->
    I could locate both start of music and end of music delimiters.
    """

    attempts = {}

    for key in _audio_sample_files:
        start_audio_fn, end_audio_fn = _audio_sample_files[key]

        start_audio = start_audio_fn()
        start_offset, start_confidence = _locate_sample(audio, start_audio)

        end_audio = end_audio_fn()
        end_offset, end_confidence = _locate_sample(audio, end_audio)

        if start_confidence < AUDIO_START_MINIMUM_CONFIDENCE or end_confidence < AUDIO_END_MINIMUM_CONFIDENCE:
            attempts[key] = (start_confidence, end_confidence)
        else:
            cut_left_offset, cut_right_offset = _adjust_cut_offset(audio,  len(start_audio), start_offset, end_offset)

            return (True, key, cut_left_offset, cut_right_offset, start_confidence, end_confidence)

    return (False, attempts)

def _adjust_cut_offset(audio, start_size, start_offset, end_offset):

    sample_rate = SUPPORTED_SAMPLE_RATE

    # because the start-of-music audio has a fade-out, it is possible (and likely) that there
    # is a tiny amount of audio from the start-of-music audio in next to audio[start_size+start_offset]
    # so it is a good idea to move a little bit to the right

    # right now we are just doing a 0.5 second cut to the right and a 0.1 cut to the left
    # but there might be better ways of doing this.

    return (start_offset + start_size + sample_rate * 0.5, end_offset - sample_rate * 0.1)


def run(file_name: str):
    print('opening', file_name)

    if not file_name.endswith('.wav'):
        print("warning: this doesn't look like a .wav file!")

    sample_rate, audio = scipy.io.wavfile.read(file_name)

    audio = _make_stereo(audio)

    if sample_rate != SUPPORTED_SAMPLE_RATE:
        print('error: the file has sample rate', sample_rate, 'but the supported sample rate is', SUPPORTED_SAMPLE_RATE)
        sys.exit(1)

    focused = focus_fg_audio(audio)

    print(focused)

if __name__ == '__main__':
    fire.Fire(run)

