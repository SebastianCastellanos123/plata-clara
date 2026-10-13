import { useCallback, useEffect, useState } from "react";
import { api, setUnauthorizedHandler } from "./api";
import AuthPage from "./components/AuthPage";
import Dashboard from "./components/Dashboard";
import type { User } from "./types";

export default function App() {
  const [user, setUser] = useState<User | null>(null);
  const [checking, setChecking] = useState(true);

  // Al cargar la página preguntamos al backend si ya hay una sesión activa
  // (la cookie viaja sola; desde JavaScript ni siquiera podemos verla).
  useEffect(() => {
    api
      .me()
      .then(setUser)
      .catch(() => setUser(null))
      .finally(() => setChecking(false));

    // Si en cualquier momento el backend dice "401", volvemos al login.
    setUnauthorizedHandler(() => setUser(null));
    return () => setUnauthorizedHandler(null);
  }, []);

  const logout = useCallback(async () => {
    try {
      await api.logout();
    } finally {
      setUser(null);
    }
  }, []);

  if (checking) return <div className="boot" role="status" aria-label="Cargando" />;
  return user ? <Dashboard user={user} onLogout={logout} /> : <AuthPage onAuth={setUser} />;
}
