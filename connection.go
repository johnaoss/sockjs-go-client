package sockjsclient

type Connection interface {
	ReadJSON(interface{}) error
	WriteJSON(interface{}) error
	Close() error
}
