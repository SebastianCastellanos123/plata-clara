-- Usuarios. Guardamos el HASH de la contraseña (bcrypt), nunca la contraseña.
CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Gastos. El monto es un entero (pesos) y NO un float: los decimales binarios
-- no representan bien el dinero (0.1 + 0.2 != 0.3). En COP no usamos centavos.
--
-- Las restricciones CHECK son una segunda línea de defensa: aunque el backend
-- tuviera un bug de validación, la base de datos rechaza datos inválidos.
CREATE TABLE expenses (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount      BIGINT NOT NULL CHECK (amount > 0),
    category    TEXT NOT NULL CHECK (category IN (
                    'comida', 'transporte', 'vivienda', 'servicios',
                    'salud', 'ocio', 'educacion', 'otros')),
    description TEXT NOT NULL DEFAULT '' CHECK (char_length(description) <= 200),
    spent_on    DATE NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Casi todas las consultas filtran por usuario y rango de fechas.
CREATE INDEX expenses_user_date_idx ON expenses (user_id, spent_on DESC);
