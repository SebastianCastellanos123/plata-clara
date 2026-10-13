import { useState, type FormEvent } from "react";
import { api, ApiError } from "../api";
import type { User } from "../types";

interface Props {
  onAuth: (user: User) => void;
}

type Mode = "login" | "register";

// Barra de ejemplo (decorativa) para que el login ya muestre de qué trata la app.
const DEMO = [
  { cat: "comida", grow: 38, label: "Comida" },
  { cat: "servicios", grow: 22, label: "Servicios" },
  { cat: "ocio", grow: 17, label: "Ocio" },
  { cat: "salud", grow: 12, label: "Salud" },
  { cat: "otros", grow: 11, label: "Otros" },
];

export default function AuthPage({ onAuth }: Props) {
  const [mode, setMode] = useState<Mode>("login");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [fields, setFields] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState(false);

  const isLogin = mode === "login";

  async function submit(e: FormEvent) {
    e.preventDefault();
    setError("");
    setFields({});
    setBusy(true);
    try {
      const user = isLogin ? await api.login(email, password) : await api.register(email, password);
      onAuth(user);
    } catch (err) {
      if (err instanceof ApiError) {
        setError(err.fields ? "" : err.message);
        setFields(err.fields ?? {});
      } else {
        setError("No pudimos conectar con el servidor. Revisa tu internet e intenta de nuevo");
      }
    } finally {
      setBusy(false);
    }
  }

  function switchMode() {
    setMode(isLogin ? "register" : "login");
    setError("");
    setFields({});
  }

  return (
    <div className="auth">
      <section className="auth-pitch">
        <p className="brand">Plata Clara</p>
        <h1>¿En qué se fue la plata este mes?</h1>
        <p className="auth-lede">Anota cada gasto en segundos y mira cómo se reparte tu mes.</p>

        <div className="demo" aria-hidden="true">
          <div className="bar">
            {DEMO.map((d) => (
              <span key={d.cat} className={`seg cat-${d.cat}`} style={{ flexGrow: d.grow }} />
            ))}
          </div>
          <ul className="demo-legend">
            {DEMO.map((d) => (
              <li key={d.cat}>
                <i className={`dot cat-${d.cat}`} />
                {d.label}
              </li>
            ))}
          </ul>
        </div>
      </section>

      <section className="auth-form">
        <h2>{isLogin ? "Inicia sesión" : "Crea tu cuenta"}</h2>

        <form onSubmit={submit} noValidate>
          <div className="field">
            <label htmlFor="email">Correo</label>
            <input
              id="email"
              type="email"
              autoComplete="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              aria-invalid={Boolean(fields.email)}
              aria-describedby={fields.email ? "email-err" : undefined}
              required
            />
            {fields.email && (
              <p id="email-err" className="field-error">
                {fields.email}
              </p>
            )}
          </div>

          <div className="field">
            <label htmlFor="password">Contraseña</label>
            <input
              id="password"
              type="password"
              autoComplete={isLogin ? "current-password" : "new-password"}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              aria-invalid={Boolean(fields.password)}
              aria-describedby={fields.password ? "password-err" : !isLogin ? "password-hint" : undefined}
              required
            />
            {!isLogin && !fields.password && (
              <p id="password-hint" className="hint">
                Mínimo 10 caracteres. Una frase larga funciona mejor que una palabra con símbolos.
              </p>
            )}
            {fields.password && (
              <p id="password-err" className="field-error">
                {fields.password}
              </p>
            )}
          </div>

          <p className="form-error" role="alert">
            {error}
          </p>

          <button className="btn primary wide" type="submit" disabled={busy}>
            {busy ? "Un momento…" : isLogin ? "Entrar" : "Crear cuenta"}
          </button>
        </form>

        <p className="switch">
          {isLogin ? "¿Aún no tienes cuenta?" : "¿Ya tienes cuenta?"}{" "}
          <button type="button" className="link" onClick={switchMode}>
            {isLogin ? "Crea una" : "Inicia sesión"}
          </button>
        </p>
      </section>
    </div>
  );
}
