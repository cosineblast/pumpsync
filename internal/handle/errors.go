
package handle

type responseError struct {
    tag string
}

func (err *responseError) Error() string {
    return err.tag
}
func newResponseError(tag string) *responseError {
    return &responseError{ tag: tag }
}

var fileTooBig = newResponseError("file_too_big")
var parseError = newResponseError("parse_error")
var negativeFileSize = newResponseError("negative_size")
var protocolViolation = newResponseError("protocol_violation")
var serverError = newResponseError("server_error")
var editFailed = newResponseError("edit_failed")
var editLocateFailed = newResponseError("edit_locate_failed")
