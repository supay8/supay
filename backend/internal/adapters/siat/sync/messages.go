package sync

import "reflect"

// toMensajes convierte la lista de mensajes del SIAT (tipo interno del SDK) a
// la representación propia de la aplicación mediante reflexión, ya que el
// paquete interno del SDK no puede importarse por nombre.
func Mensajes(msgs any) []Mensaje {
	v := reflect.ValueOf(msgs)
	if !v.IsValid() || v.Kind() != reflect.Slice {
		return nil
	}
	out := make([]Mensaje, 0, v.Len())
	for i := 0; i < v.Len(); i++ {
		elem := v.Index(i)
		if elem.Kind() == reflect.Pointer {
			elem = elem.Elem()
		}
		var m Mensaje
		if f := elem.FieldByName("Codigo"); f.IsValid() {
			m.Codigo = int(f.Int())
		}
		if f := elem.FieldByName("Descripcion"); f.IsValid() {
			m.Descripcion = f.String()
		}
		out = append(out, m)
	}
	return out
}

func toMensajes(msgs any) []Mensaje { return Mensajes(msgs) }
