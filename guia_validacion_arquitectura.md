# Guía de Inspección y Validación Técnica de la Arquitectura OKF Converter

Esta guía detalla los comandos, consultas SQL y herramientas visuales para inspeccionar y verificar cada componente de la infraestructura desplegada con **Docker Compose**. Es ideal para demostraciones en la sustentación y pruebas de robustez del sistema.

---

## 🏛️ Resumen de Puertos y Dashboards de Administración

| Componente | URL / Dirección | Credenciales por Defecto | Propósito de Inspección |
| :--- | :--- | :--- | :--- |
| **Frontend Web** | [http://localhost:3000](http://localhost:3000) | N/A (Registro libre) | Interfaz gráfica SPA de usuario final. |
| **API REST (Go)** | [http://localhost:8080/api/health](http://localhost:8080/api/health) | Bearer Token (JWT) | Endpoints HTTP *stateless* y salud del servicio. |
| **Dashboard RabbitMQ** | [http://localhost:15672](http://localhost:15672) | `guest` / `guest` | Monitoreo visual de la cola de mensajes y tasa de entrada/salida. |
| **Console MinIO (S3)** | [http://localhost:9001](http://localhost:9001) | `minioadmin` / `minioadmin` | Inspección gráfica del bucket de almacenamiento no efímero. |
| **Base de Datos Postgres** | `localhost:5432` (o via `docker exec`) | `okf_user` / `okf_password` | Consultas SQL directas a tablas de usuarios, jobs y logs. |

---

## 1. Validación de la Base de Datos (PostgreSQL)

Para verificar la persistencia relacional, estados de los trabajos y registros de auditoría:

### Conexión Interactiva a psql dentro del Contenedor
Ejecuta el siguiente comando en la terminal:

```bash
docker exec -it okf_postgres psql -U okf_user -d okf_db
```

### Consultas SQL Clave de Verificación

#### A. Ver Usuarios Registrados
```sql
SELECT id, email, created_at FROM users;
```

#### B. Ver Estado Actual de los Trabajos (`jobs`)
```sql
SELECT id, user_id, original_filename, status, units_count, created_at, updated_at 
FROM jobs 
ORDER BY created_at DESC;
```

#### C. Ver Registro de Auditoría y Trazabilidad (`log.md` en BD)
Reemplaza `'<JOB_ID>'` por el ID del trabajo a inspeccionar:
```sql
SELECT step, status, message, created_at 
FROM bundle_logs 
WHERE job_id = '<JOB_ID>' 
ORDER BY created_at ASC;
```

#### D. Salir de psql
```sql
\q
```

---

## 2. Inspección de la Cola de Mensajes (RabbitMQ)

Para verificar el desacoplamiento asíncrono y la entrega de tareas:

### A. Inspección Gráfica (Dashboard Web)
1. Abre tu navegador en [http://localhost:15672](http://localhost:15672).
2. Ingresa con usuario `guest` y contraseña `guest`.
3. Ve a la pestaña **Queues** y selecciona la cola `okf_jobs`.
4. Observa:
   * **Ready:** Mensajes en espera de ser consumidos.
   * **Unacked:** Mensajes siendo procesados actualmente por un Worker Go.
   * **Total:** Tasa de entrada y salida (*Message rates*).
   * **Consumers:** Lista de instancias de Workers Go conectadas activamente.

### B. Prueba de Escalabilidad de Workers
Puedes escalar el número de trabajadores en paralelo para procesar múltiples mensajes simultáneamente:
```bash
docker compose up --scale worker=3 -d
```
En el dashboard de RabbitMQ verás inmediatamente que la sección **Consumers** incrementa a **3**.

---

## 3. Validación del Almacenamiento de Objetos (MinIO - S3)

Para comprobar que los archivos originales y los bundles OKF persisten fuera del disco efímero de los contenedores:

1. Abre tu navegador en [http://localhost:9001](http://localhost:9001).
2. Inicia sesión con `minioadmin` / `minioadmin`.
3. Selecciona el bucket **`okf-bundles`** en el menú de la izquierda.
4. Navega en la estructura de carpetas:
   * **`originals/{USER_ID}/`**: Archivos subidos originalmente por los usuarios (`.md`, `.docx`, `.pdf`, etc.).
   * **`bundles/{USER_ID}/`**: Paquetes finales comprimidos `.zip` validados y listos para descarga.
5. Puedes descargar o previsualizar directamente cualquier archivo subido desde el navegador.

---

## 4. Inspección de Logs de la API Go y los Workers

Para monitorear el comportamiento en tiempo real desde la consola:

### A. Logs de la API REST (Recepción HTTP e Encolamiento)
```bash
docker logs -f okf_api
```
*Verás cuando la API recibe las peticiones, firma tokens JWT, guarda en MinIO y responde en milisegundos.*

### B. Logs del Worker Go (Consumo, Conversión y Validación OKF)
```bash
docker logs -f okf_worker
```
*Verás cómo el Worker consume el evento de RabbitMQ, procesa el texto, segmenta unidades, valida los enlaces del `index.md` y registra la idempotencia.*

---

## 5. Guía Paso a Paso para la Demostración en la Sustentación

Sigue este flujo para demostrar el cumplimiento de las 6 condiciones verificables de la rúbrica:

### Paso 1: Levantar el Entorno Limpio
```bash
docker compose up --build
```
*Muestra en consola cómo se inicializan PostgreSQL, RabbitMQ, MinIO, API Go, Worker Go y Frontend Nginx.*

### Paso 2: Probar Asincronía Efectiva
1. Abre [http://localhost:3000](http://localhost:3000) e inicia sesión.
2. Sube un documento estructurado de varias páginas.
3. Muestra cómo la interfaz recibe de inmediato el `job_id` con estado `PENDING`.
4. Recarga la página o abre los logs del Worker (`docker logs -f okf_worker`) para evidenciar que el procesamiento continuó sin bloquear la conexión HTTP.

### Paso 3: Probar Aislamiento Multiusuario (Multitenancy)
1. Inicia sesión en una pestaña de incógnito con un **Usuario B**.
2. Intenta acceder por URL o API al `job_id` generado por el **Usuario A**.
3. Muestra la respuesta en pantalla: **HTTP 403 Forbidden (Acceso denegado)**.

### Paso 4: Previsualizar e Inspeccionar el Bundle OKF Generado
1. En la tabla del Frontend, haz clic en el botón **"Logs"**.
2. Observa la previsualización directa del contenido de `log.md` y sus marcas de tiempo.
3. Presiona **"Descargar Bundle OKF (.zip)"**, descomprime el archivo y abre `index.md` comprobando que los hipervínculos dirigen correctamente a los archivos `.md` de concepto.
