import { LogoSupay } from '@/components/logo';
import React, { useState } from 'react';

export default function LoginPage() {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [isLoading, setIsLoading] = useState(false);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setIsLoading(true);
    // Simulación de petición de autenticación
    setTimeout(() => {
      setIsLoading(false);
    }, 1200);
  };

  return (
    <div className="min-h-screen flex items-center justify-center bg-background px-4 relative overflow-hidden">
      {/* Fondo de patrón de rejilla sutil opcional (estilo infraestructura) */}
      <div 
        className="absolute inset-0 opacity-[0.03] dark:opacity-[0.05] pointer-events-none" 
        style={{
          backgroundImage: `radial-gradient(var(--text) 1px, transparent 1px)`,
          backgroundSize: '24px 24px'
        }}
      />

      <div className="w-full max-w-sm relative z-10">
        {/* Encabezado / Logo marca */}
        <div className="flex flex-col items-center mb-8">
             <LogoSupay size={50} className="shrink-0 text-primary"  />
          <h1 className="text-xl font-medium tracking-tight text-foreground">
            Acceder a Supay
          </h1>
          <p className="text-sm text-muted-foreground mt-1 font-mono">
            infraestructura.v1
          </p>
        </div>

        {/* Tarjeta de Formulario */}
        <div className="bg-card border border-border rounded-xl p-6 shadow-2xl shadow-black/5 dark:shadow-black/40">
          <form onSubmit={handleSubmit} className="space-y-4">
            
            {/* Campo Email */}
            <div className="space-y-1.5">
              <label 
                htmlFor="email" 
                className="block text-xs font-mono uppercase tracking-wider text-muted-foreground"
              >
                Correo electrónico
              </label>
              <input
                id="email"
                type="email"
                required
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                placeholder="ingeniero@empresa.com"
                className="w-full h-10 px-3 bg-background border border-input rounded-md text-sm text-foreground placeholder:text-muted-foreground/50 focus:outline-none focus:border-primary focus:ring-1 focus:ring-primary transition-all font-mono"
              />
            </div>

            {/* Campo Contraseña */}
            <div className="space-y-1.5">
              <div className="flex items-center justify-between">
                <label 
                  htmlFor="password" 
                  className="block text-xs font-mono uppercase tracking-wider text-muted-foreground"
                >
                  Contraseña
                </label>
                <a 
                  href="#forgot" 
                  className="text-xs text-muted-foreground hover:text-foreground transition-colors font-mono"
                >
                  ¿Olvidaste?
                </a>
              </div>
              <input
                id="password"
                type="password"
                required
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder="••••••••••••"
                className="w-full h-10 px-3 bg-background border border-input rounded-md text-sm text-foreground placeholder:text-muted-foreground/50 focus:outline-none focus:border-primary focus:ring-1 focus:ring-primary transition-all font-mono"
              />
            </div>

            {/* Botón de acción principal */}
            <button
              type="submit"
              disabled={isLoading}
              className="w-full mt-2 h-10 bg-primary text-primary-foreground font-medium rounded-md hover:bg-primary-hover active:scale-[0.99] transition-all flex items-center justify-center text-sm shadow-sm disabled:opacity-50 disabled:pointer-events-none cursor-pointer"
            >
              {isLoading ? (
                <span className="inline-block w-4 h-4 border-2 border-primary-foreground border-t-transparent rounded-full animate-spin" />
              ) : (
                'Iniciar sesión'
              )}
            </button>
          </form>
        </div>

        {/* Footer discreto */}
        <div className="text-center mt-6">
          <p className="text-xs text-muted-foreground font-mono">
            ¿No tienes cuenta?{' '}
            <a href="/signup" className="text-foreground hover:underline underline-offset-4">
              Crear cuenta ahora
            </a>
          </p>
        </div>
      </div>
    </div>
  );
}