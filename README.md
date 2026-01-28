# Os File Poller

This is a low level thread pooling package built around the linux epoll7 api.  The primary objective of this package 
is to allow managing multiple data streams from a single thread or a small pool of threads.  The development process
began as a way to manage eternal process based ffmpeg transcoding in conjunction with grpc-web video data web streams and plug them into Vosk and MellowTTS container http services.

Thread pooling of the following is currently supported:
  - Process watchers
  - Scheduled callbacks
  - Intervall callbacks
  - Timeout callbacks
  - Read + timeout callbacks
  - Write + timeout callbacs
