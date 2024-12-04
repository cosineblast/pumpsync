#!/usr/bin/env python3

import numpy as np

import scipy.io.wavfile as wavfile
import scipy.signal
import json

import fire

def locate_audio(haystack, needle):
    """
    Tries to find the best position for the audio `needle` in `haystack`.
    Returns (start_offset, confidence)
    """

    correlation = scipy.signal.correlate(haystack, needle)

    audio_start = int(np.argmax(correlation)) - len(needle) + 1

    z_score = (np.max(correlation)- np.mean(correlation)) / np.std(correlation)

    return (audio_start, float(z_score))

SUPPORTED_SAMPLE_RATE = 48_000

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
         'score': score,
         'needle_duration': len(needle) / SUPPORTED_SAMPLE_RATE }))

if __name__ == '__main__':
    fire.Fire(main)

