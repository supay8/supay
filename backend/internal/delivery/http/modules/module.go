package modules

import "github.com/go-chi/chi/v5"

// Module es un contrato que expone un dominio de la aplicación. Cada módulo
// autónomo registra sus propias rutas bajo el router que recibe, lo que
// permite que el router principal sea un coordinador puro.
type Module interface {

	// RegisterRoutes registra las rutas del módulo en el router dado.
	RegisterRoutes(r chi.Router)
	// PathPrefix devuelve el prefijo de ruta bajo el cual se montará el
	// módulo. Una cadena vacía significa que se registran en el router raíz.
	PathPrefix() string
}
