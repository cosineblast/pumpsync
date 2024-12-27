///
/// This module implements an executable tailored for the detection of the offset of audio in another one,
/// for the pumpsync backend. The program receives the path of two mono channel wav files from the command
/// line, known as 'haystack' and 'needle' respectively, and tries to determine when does 
/// 'needle' play in 'haystack'.
///
/// One of its main goals is to use less than 512MiB of RAM when given two 44.1k .wav files with 3
/// minutes of duration less, so that it is possible to run it within a constricted memory
/// environment.


use std::{fs::File, io::{BufReader, BufWriter, Read, Seek, Write}};

use hound::WavReader;
use rustfft::{algorithm::Radix4, num_complex::{Complex, Complex32}, Fft, FftDirection, FftNum};
use tempfile::tempfile;

trait BasicallyAFloat : PartialOrd + FftNum + From<f32> + Default + Into<f64> {
}

impl BasicallyAFloat for f32 { }
impl BasicallyAFloat for f64 { }

fn find_target_size(my_size: usize, other_size: usize) -> usize {
    let n = my_size + other_size  - 1;

    n.next_power_of_two()
}

fn read_and_pad<T>(reader: &mut WavReader<BufReader<File>>, my_size: usize, other_size: usize) -> Vec<Complex<T>>
    where T: BasicallyAFloat
{
    assert_eq!(reader.duration(), my_size as u32);

    let target_buffer_size = find_target_size(my_size, other_size);

    let mut buffer: Vec<Complex<T>> =
        Vec::with_capacity(target_buffer_size);

    for sample in sample_iterator(reader) {
        buffer.push(Complex{ re: T::from(sample), im: T::default() })
    }

    for _ in 0..(target_buffer_size - my_size) {
        buffer.push(Complex{ re: T::default(), im: T::default() })
    }

    assert_eq!(buffer.len(), buffer.capacity());

    return buffer;
}

// we can't just return a non-heaped iterator because we return different iterator types
// for depending on wether it is an integer wav or a floating wav
fn sample_iterator<'a>(reader: &'a mut WavReader<BufReader<File>>) -> Box<dyn Iterator<Item = f32> + 'a> {

    let spec = reader.spec();

    // right now our backend will just make sure the files are mono, but in the future we may
    // want to convert it to mono ourselves by averaging the channels
    assert_eq!(spec.channels, 1, "this program does not support stereo channels");

    let thing: Box<dyn Iterator<Item = f32>> = match spec.sample_format {
        hound::SampleFormat::Float => {
            Box::new(reader.samples::<f32>().map(|sample| sample.unwrap()))
        },
        hound::SampleFormat::Int =>
            Box::new(reader.samples::<i32>().map(|sample| sample.unwrap() as f32))
    };

    thing
}


fn freeze(buffer: Vec<Complex32>) -> std::io::Result<File> {

    eprintln!("freezing {} elements...\n", buffer.len());

    let mut file = BufWriter::new(tempfile()?);

    file.write_all(&buffer.len().to_le_bytes())?;

    for value in buffer {
        let real_bytes = value.re.to_le_bytes();
        let imaginary_bytes = value.im.to_le_bytes();

        file.write_all(&real_bytes)?;
        file.write_all(&imaginary_bytes)?;
    }

    file.rewind()?;

    eprintln!("freezing done\n");

    return Ok(file.into_inner()?)
}

fn unfreeze(file: File) -> std::io::Result<Vec<Complex<f32>>> {

    let mut file = BufReader::new(file);

    eprintln!("unfreezing...\n");

    let mut buf = [0u8; 8];

    file.read_exact(&mut buf)?;

    let len = usize::from_le_bytes(buf);

    eprintln!("reading {} elements...\n", len);

    let mut result = Vec::with_capacity(len);

    for _ in 0..len {
        let mut real_bytes = [0u8; 4];
        let mut imaginary_bytes = [0u8; 4];

        file.read_exact(&mut real_bytes)?;
        file.read_exact(&mut imaginary_bytes)?;

        let real = f32::from_le_bytes(real_bytes);
        let imaginary = f32::from_le_bytes(imaginary_bytes);

        result.push(Complex{ re: real, im: imaginary});
    }

    assert_eq!(result.capacity(), len);
    assert_eq!(result.len(), len);

    eprintln!("unfreezing done\n");

    Ok(result)
}

fn compute_fft(mut buffer: Vec<Complex<f32>>, direction: rustfft::FftDirection) -> Vec<Complex<f32>> {
    let n = buffer.len();

    let forward = Radix4::<f32>::new(n, direction);

    let helper_size = forward.get_inplace_scratch_len();

    assert_eq!(helper_size, n);

    let mut scratch: Vec<Complex<f32>> = Vec::with_capacity(helper_size);

    scratch.resize(helper_size, Complex::default());

    assert_eq!(scratch.capacity(), helper_size);

    forward.process_with_scratch(&mut buffer, &mut scratch);

    return buffer
}

// receives two vectors of complex values which are in frequency domain, and computes the correlation for them
fn compute_correlation_post_fft(mut haystack_buffer: Vec<Complex<f32>>, needle_buffer: Vec<Complex<f32>> ,
    haystack_sample_count: usize,
    needle_sample_count: usize) -> Vec<Complex<f32>>
{

    assert_eq!(haystack_buffer.len(), needle_buffer.len());
    let n = haystack_buffer.len();

    for i in 0..n {
        haystack_buffer[i] =
            haystack_buffer[i] *
            needle_buffer[i]
            / (n as f32);
    }

    drop(needle_buffer);

    let mut result = compute_fft(haystack_buffer, rustfft::FftDirection::Inverse);

    result.truncate(haystack_sample_count + needle_sample_count - 1);

    return result;
}

fn compute_mean_stddev_re<F>(stuff: &[Complex<F>]) -> (f64, f64)
    where F: BasicallyAFloat {
    let mean = stuff.iter().map(|it| it.re.into() as f64).sum::<f64>() / (stuff.len() as f64);

    let variance = stuff.iter().map(|it| {
        let difference = it.re.into() as f64 - mean;

        difference * difference
    }).sum::<f64>() / (stuff.len() as f64);

    (mean, variance.sqrt())
}

fn locate_audio_start(correlation: &[Complex<f32>],
    needle_sample_count: usize,
    haystack_sample_count: usize,
    sample_rate: usize) -> (usize, f64) where {

    let min_correlation =
        correlation.iter()
        .enumerate()
        .min_by(|l, r| l.1.re.partial_cmp(&r.1.re).unwrap())
        .unwrap();

    let fraction_of_second = sample_rate / 10;

    let max_post_min_correlation =
        correlation[min_correlation.0..min_correlation.0+fraction_of_second].iter()
        .enumerate()
        .max_by(|l, r| l.1.re.partial_cmp(&r.1.re).unwrap())
        .unwrap();

    let max_correlation =
        correlation.iter()
        .max_by(|l, r| l.re.partial_cmp(&r.re).unwrap())
        .unwrap();

    // we convert it to i64 because sometimes calcuations go wrong and give us a
    // negative audio start number. we should react to this accordingly
    let audio_start = (min_correlation.0 + max_post_min_correlation.0) as i64 - needle_sample_count as i64 + 1;

    // now, we need to give a confidence score to our guess,
    let (mean, stddev) = compute_mean_stddev_re(&correlation);

    let z_score = (max_correlation.re as f64 - mean) / stddev;

    // sometimes calculations go really wrong and we end up estimating
    // that the audio starts somewhere impossible
    // in that case, the right thing to do is to claim that we have absolutely
    // no confidence on the result
    if audio_start < 0 || audio_start as usize >= haystack_sample_count {
        return (0, 0.0)
    }

    return (audio_start as usize, z_score)
}

fn main() {

    let mut args = std::env::args();

    args.next();
    let haystack_path = args.next().unwrap();
    let needle_path = args.next().unwrap();

    //

    let mut haystack_reader = hound::WavReader::open(haystack_path).unwrap();

    let mut needle_reader = hound::WavReader::open(needle_path).unwrap();

    let sample_rate = haystack_reader.spec().sample_rate;

    assert_eq!(sample_rate, needle_reader.spec().sample_rate, "Files have non matching sample rates");

    let haystack_sample_count = haystack_reader.duration() as usize;

    let needle_sample_count = needle_reader.duration() as usize;

    let haystack_buffer = read_and_pad::<f32>(&mut haystack_reader, haystack_sample_count, needle_sample_count);

    let haystack_fft = compute_fft(haystack_buffer, FftDirection::Forward);

    let frozen_haystack_fft = freeze(haystack_fft).unwrap();

    let mut needle_buffer = read_and_pad::<f32>(&mut needle_reader, needle_sample_count, haystack_sample_count);

    needle_buffer[0..needle_sample_count].reverse();

    let needle_fft = compute_fft(needle_buffer, FftDirection::Forward);

    let haystack_fft = unfreeze(frozen_haystack_fft).unwrap();

    let correlation = compute_correlation_post_fft(haystack_fft, needle_fft, haystack_sample_count, needle_sample_count);

    let (audio_start_sample, score) = locate_audio_start(&correlation, needle_sample_count, haystack_sample_count, sample_rate as usize);

    let audio_start = audio_start_sample as f64 / (sample_rate as f64);

    // todo: use serde?
    println!(r#" {{"offset":{}, "score": {} }}"#,
        audio_start,
        score);
}

#[cfg(test)]
mod test {
}
