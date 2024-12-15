
package video_store

import (
    "github.com/google/uuid"
    "os"
    "time"
	"sync"
)

type VideoStore struct {
    availableVideos sync.Map
}

func NewVideoStore() VideoStore {
    return VideoStore{}
}

func (store *VideoStore) FetchVideo(id uuid.UUID) *string {
    value, ok := store.availableVideos.Load(id)

    if !ok {
        return nil
    }

    result := new(string)
    *result = value.(string)
    return result
}


// Moves the file in the given file to the video store 
// the file will be automatically removed from the store after 5 minutes.
func (store *VideoStore) AddVideo(path string) (uuid.UUID, error) {

    var err error

    defer func() {
        if err != nil {
            os.Remove(path)
        }
    }()

    uid, err := uuid.NewRandom()

    if err != nil {
        return uuid.UUID{}, err
    }

    file, err := os.CreateTemp("", "pumsync_result_*.mp4")

    if err != nil {
        return uuid.UUID{}, err
    }

    file.Close()

    err = os.Rename(path, file.Name())

    if err != nil {
        return uuid.UUID{}, err
    }

    store.availableVideos.Store(uid, file.Name())

    go func() {
        time.Sleep(time.Duration(5 * time.Minute))

        store.availableVideos.Delete(uid)
        os.Remove(file.Name())
    }()

    return uid, nil
}
