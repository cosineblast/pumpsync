#!/usr/bin/env python3

import numpy as np

import scipy.io.wavfile as wavfile
import scipy.signal
import json

import fire

SUPPORTED_SAMPLE_RATE = 48_000

def locate_audio(haystack, needle):
    """
    Tries to find the best position for the audio `needle` in `haystack`.
    Returns (start_offset, confidence)
    """

    correlation = scipy.signal.correlate(haystack, needle)

    lowest_point = np.argmin(correlation)

    # our heuristic is to find the lowest point of correlation then
    # find the highest point near that one.
    highest_point = lowest_point + np.argmax(correlation[lowest_point:int(lowest_point+SUPPORTED_SAMPLE_RATE * 0.1)])

    audio_start = highest_point - len(needle) + 1

    z_score = (np.max(correlation)- np.mean(correlation)) / np.std(correlation)

    # sometimes things go really wrong and we end up estimating
    # that the audio starts somewhere impossible
    # in that case, the right thing to do is to claim that we have absolutely
    # no confidence on the result
    if audio_start < 0 or audio_start >= len(haystack):
        return (0, 0)

    return (audio_start, float(z_score))


def _get_audio(path):
    sample_rate, data = scipy.io.wavfile.read(path)

    assert sample_rate == SUPPORTED_SAMPLE_RATE

    return _make_stereo(data.astype('float64'))

def _make_stereo(data):
    if len(data.shape) == 2 and data.shape[1] == 2:
        transposed = np.transpose(data)
        return (transposed[0] + transposed[1]) / 2.0

    return data

def main(haystack_path: str, needle_path: str):
    """
    Tries to locate the position of an audio file inside another one.

    haystack_path and needle_path must be strings pointing to the path of .wav files with a sample rate of 48k.

    This scripts emits in stdout a json dictionary with three keys:
    `offset`: a floating point value indicating the location of the `needle` file in the `haystack` file, in seconds
    `needle_duration`: a floating point value indicating the duration of the needle, in seconds
    `score`: a floating point number that represents the confidence of this script with the given input.
    """

    haystack = _get_audio(haystack_path)

    needle = _get_audio(needle_path)

    start, score = locate_audio(haystack, needle)

    print(json.dumps(
        {'offset': start / SUPPORTED_SAMPLE_RATE,
         'score': score }))

if __name__ == '__main__':
    fire.Fire(main)

