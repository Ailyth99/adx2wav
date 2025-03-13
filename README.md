
# ADX2WAV

Simple tool to convert CRI ADX audio files to WAV.  It's written in Go, doesn't need a bunch of extra stuff installed.  Useful if you're messing with game audio or something.


## Usage

```bash
adx2wav <input.adx> [output.wav]
```

*   `<input.adx>`: The ADX file you want to convert.
*   `[output.wav]` (Optional): Where you want to save the WAV.  

**Example:**

```bash
adx2wav sound.adx          # Converts to sound.wav
adx2wav music.adx out.wav  # Converts to out.wav
```

## Building

Need Go installed. Then:

Build:

    ```bash
    go build
    ```

This makes the `adx2wav` executable.
