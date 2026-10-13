# ---------- 1) Compilar el frontend ----------
FROM node:22-alpine AS web
WORKDIR /web
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# ---------- 2) Compilar el backend ----------
FROM golang:1.24-alpine AS api
WORKDIR /src
COPY backend/ ./
# "go mod tidy" descarga las dependencias y crea go.sum si aún no lo has commiteado.
RUN go mod tidy && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

# ---------- 3) Imagen final, mínima ----------
# "distroless": sin shell, sin gestor de paquetes, sin nada que un atacante pueda usar.
# Corre como usuario sin privilegios (nonroot).
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=api /out/api /app/api
COPY --from=web /web/dist /app/public
ENV STATIC_DIR=/app/public PORT=8080
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/api"]
