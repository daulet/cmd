# Transcribe Youtube videos

```
youtube-dl -f "worst" "https://www.youtube.com/watch?v=l8pRSuU81PU"
ffmpeg \
    -i video.mp4 \
    -ar 16000 \
    -ac 1 \
    -map 0:a: \
    audio.mp3

# split into chunks of 15 minutes, too many files and we run into rate limits
ffmpeg -i audio.mp3 -f segment -segment_time 900 -c copy intervals/out%03d.mp3

go run ./examples/transcribe ./intervals
```