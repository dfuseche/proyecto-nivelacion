# Plataforma de Conversión Documental a Bundles OKF

**Asignatura:** ISIS4426 - Desarrollo de Soluciones Cloud  
**Modalidad:** Proyecto de Nivelación  

Este repositorio contiene la solución completa de arquitectura en la nube para la conversión asíncrona de documentos en **Bundles de Conocimiento compatibles con Open Knowledge Format (OKF)**.

---

## 🚀 Arquitectura y Tecnologías

El sistema está diseñado bajo una arquitectura de microservicios desacoplados y orientados a eventos, orquestados mediante **Docker Compose**:

* **Backend API (Go):** API REST *stateless* que recibe archivos, los guarda en el almacenamiento de objetos, registra el trabajo en PostgreSQL y encola un evento en RabbitMQ respondiendo de inmediato al cliente.
* **Worker (Go):** Proceso independiente que consume tareas de RabbitMQ, descarga el archivo original de MinIO, segmenta las unidades lógicas, genera `index.md`, `log.md` y los conceptos en Markdown, valida la integridad del bundle y persiste el paquete ZIP.
* **Cola de Mensajes (RabbitMQ):** Garantiza desacoplamiento total y procesamiento asíncrono no bloqueante con confirmaciones (ACKs) e idempotencia.
* **Base de Datos (PostgreSQL):** Persistencia de usuarios, trabajos, estados y trazabilidad (`bundle_logs`).
* **Object Storage (MinIO):** Almacenamiento de archivos originales y bundles fuera del disco efímero de los contenedores.
* **Frontend Web (HTML5 / Vanilla CSS / JS):** Interfaz SPA con estética *glassmorphism* (modo oscuro), monitoreo en tiempo real, visor de trazabilidad y descargas.

---

## 📦 Instrucciones de Despliegue con Un Solo Comando

### Requisitos Previos
* Docker y Docker Compose instalados en el sistema.

### Pasos para Desplegar

1. Clonar este repositorio y ubicarse en la raíz del proyecto.
2. Asegurarse de tener el archivo `.env` configurado (se incluye `.env.example` de referencia).
3. Ejecutar el siguiente comando único para construir y levantar los 6 servicios:

```bash
docker compose up --build
```

4. Abrir el navegador en los siguientes puertos:
   * **Frontend Web:** [http://localhost:3000](http://localhost:3000)
   * **API REST (Go):** [http://localhost:8080/api/health](http://localhost:8080/api/health)
   * **Dashboard RabbitMQ:** [http://localhost:15672](http://localhost:15672) (Usuario/Clave: `guest` / `guest`)
   * **Dashboard MinIO:** [http://localhost:9001](http://localhost:9001) (Usuario/Clave: `minioadmin` / `minioadmin`)

---

## 🔍 Verificación del Cumplimiento de Requisitos

| Condición | Cómo Verificarla en el Sistema |
| :--- | :--- |
| **1. Asincronía Efectiva** | Subir un archivo desde el Frontend. La API responde inmediatamente con estado `PENDING`. Se puede cerrar la ventana/conexión y el Worker continuará el trabajo. |
| **2. Documento Breve** | Cargar un archivo sin encabezados. Se genera un bundle válido con `index.md`, `log.md` y 1 único concepto (`documento.md`). |
| **3. Documento Estructurado** | Cargar un archivo con múltiples `# Encabezados`. Se generan archivos individualizados (`capitulo-01.md`, etc.) ordenados e hipervinculados desde `index.md`. |
| **4. Bundle Incompleto** | Si falta `index.md` o `log.md`, el validador marca el trabajo como `INVALID`, bloqueando su publicación y descarga. |
| **5. Aislamiento Multiusuario** | Intentar consultar o descargar un `job_id` perteneciente al Usuario A utilizando la sesión del Usuario B retorna `403 Forbidden`. |
| **6. Idempotencia** | Si RabbitMQ entrega el mismo mensaje dos veces, el Worker verifica en PostgreSQL si el trabajo ya está `COMPLETED` y omite el duplicado. |

---

## 🛠️ Estructura del Código Fuente Go

```
.
├── cmd/
│   ├── api/main.go            # Punto de entrada de la API REST HTTP
│   └── worker/main.go         # Punto de entrada del Worker consumidor
├── internal/
│   ├── config/config.go       # Carga de variables de entorno
│   ├── converter/okf.go       # Motor de segmentación y conversión OKF
│   ├── domain/models.go       # Modelos de datos, DTOs y claims JWT
│   ├── repository/            # Conexiones DB (Postgres), Storage (MinIO) y Queue (RabbitMQ)
│   ├── service/               # Lógica de negocio (Auth, Jobs, Aislamiento)
│   └── validator/okf.go       # Validador de estructura e hipervínculos
├── frontend/                  # Interfaz de usuario SPA (Nginx)
├── docker-compose.yml         # Orquestación de los 6 contenedores
├── init.sql                   # Esquema inicial de PostgreSQL
└── README.md                  # Guía de despliegue y pruebas
```
