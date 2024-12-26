
run: server locate_audio
	go run .

server:
	go build .

# we generally use the release version of the locate_audio program
# because the debug is often reeeally slow
locate_audio:
	cd locate; \
	  cargo build --release
	cp locate/target/release/ps_locate locate_audio



.PHONY: server locate_audio

