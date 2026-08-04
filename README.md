# 👹 Supay

<div align="center">

<img src="https://raw.githubusercontent.com/devicons/devicon/master/icons/typescript/typescript-original.svg" width="80" alt="TypeScript"/>

### Infraestructura Open Source para Facturación Electrónica en Bolivia 🇧🇴

**SIAT · SOAP · XML · CUIS · CUFD · NestJS · PostgreSQL**

<p align="center">
  <img src="https://img.shields.io/badge/Bolivia-SIAT-red?style=for-the-badge&logo=data:image/svg+xml;base64,PHN2Zy8+" alt="Bolivia SIAT"/>
  <img src="https://img.shields.io/badge/NestJS-E0234E?style=for-the-badge&logo=nestjs&logoColor=white" alt="NestJS"/>
  <img src="https://img.shields.io/badge/TypeScript-3178C6?style=for-the-badge&logo=typescript&logoColor=white" alt="TypeScript"/>
  <img src="https://img.shields.io/badge/PostgreSQL-4169E1?style=for-the-badge&logo=postgresql&logoColor=white" alt="PostgreSQL"/>
  <img src="https://img.shields.io/badge/Prisma-2D3748?style=for-the-badge&logo=prisma&logoColor=white" alt="Prisma"/>
  <img src="https://img.shields.io/badge/License-MIT-green?style=for-the-badge" alt="MIT"/>
</p>

---

**Supay simplifica la integración con el SIAT para que puedas construir sistemas de facturación electrónica modernos sin pelearte con SOAP, XML y toda la burocracia técnica del SIN.**

</div>

---

## ✨ ¿Qué es Supay?

Supay es un **SDK + plataforma backend** para Bolivia que abstrae la complejidad de la facturación electrónica:

<table>
<tr>
<td width="50%">

### 😵 Sin Supay

* XML manual
* SOAP complejo
* Gestión de CUIS/CUFD
* Firmas digitales
* Contingencias
* Catálogos SIAT
* Errores difíciles de depurar

</td>
<td width="50%">

### 😎 Con Supay

* API moderna
* Tipado con TypeScript
* Generación automática de XML
* Cliente SIAT reutilizable
* Tests y simulaciones
* Arquitectura escalable
* Integración sencilla

</td>
</tr>
</table>

---

## 🏗️ Arquitectura

```mermaid
flowchart LR
    A[Tu sistema] -- REST/SDK --> B[Supay API]
    B --> C[Motor XML]
    B --> D[Gestor CUIS/CUFD]
    B --> E[Firma digital]
    C --> F[SIAT SOAP]
    D --> F
    E --> F
```

---

## 🚀 Características

### 🔌 Integración SIAT

* ✅ Cliente SOAP modular
* ✅ Obtención automática de **CUIS**
* ✅ Gestión automática de **CUFD**
* ✅ Sincronización de catálogos
* 🚧 Recepción de facturas
* 🚧 Anulación y eventos significativos
* 🚧 Contingencias offline

### 🛠️ Backend moderno

* ⚡ **NestJS**
* 🔷 **TypeScript**
* 🗄️ **Prisma ORM**
* 🐘 **PostgreSQL**
* 📦 **pnpm + Turbo**
* 🐳 **Docker**

### 🧪 Preparado para desarrollo

* ✅ Mocks SIAT
* ✅ Scripts de simulación
* ✅ Tests de integración
* ✅ Separación entre lógica fiscal y negocio

---

## 📁 Estructura del proyecto

```text
supay/
├── apps/
│   ├── backend/          # API principal NestJS
│   ├── frontend/         # Dashboard (futuro)
│   └── docs/
│
├── packages/
│   ├── siat-sdk/         # Cliente SIAT
│   ├── xml-builder/      # Generador XML
│   └── shared/           # Tipos compartidos
│
├── docker/
│   └── docker-compose.yml
│
├── turbo.json
├── pnpm-workspace.yaml
└── README.md
```

---

## ⚡ Inicio rápido

### 1️⃣ Clonar

```bash
git clone https://github.com/tu-org/supay.git
cd supay
```

### 2️⃣ Instalar dependencias

```bash
pnpm install
```

### 3️⃣ Levantar PostgreSQL

```bash
docker compose up -d
```

### 4️⃣ Configurar entorno

```bash
cp apps/backend/.env.example apps/backend/.env
```

```env
DATABASE_URL="postgresql://postgres:postgres@localhost:5432/supay"
NODE_ENV=development
```

### 5️⃣ Ejecutar migraciones

```bash
cd apps/backend
pnpm prisma migrate dev
```

### 6️⃣ Iniciar el servidor

```bash
pnpm start:dev
```

<div align="center">

### 🌐 API disponible en

## `http://localhost:3000`

</div>

---

## 🧪 Filosofía actual: **Test First**

Por ahora **NO estamos integrando servicios productivos del SIAT directamente en los módulos de negocio**.

La estrategia es:

```text
Scripts SIAT
      ↓
Simulación local
      ↓
Tests de integración
      ↓
Validación de flujos
      ↓
Integración real
```

### Ejecutar tests

```bash
pnpm test
```

### Ejecutar simulaciones SIAT

```bash
pnpm test:siat
```

Esto permite desarrollar **sin credenciales reales ni dependencia del entorno del SIN**.

---

## 📸 Flujo esperado

```text
┌─────────────┐
│ Crear venta │
└──────┬──────┘
       │
       ▼
┌─────────────┐
│ Generar XML │
└──────┬──────┘
       │
       ▼
┌─────────────┐
│ Firmar XML  │
└──────┬──────┘
       │
       ▼
┌─────────────┐
│ Obtener CUFD│
└──────┬──────┘
       │
       ▼
┌─────────────┐
│ Enviar SIAT │
└──────┬──────┘
       │
       ▼
┌─────────────┐
│ Respuesta   │
└─────────────┘
```

---

## 🗺️ Roadmap

### 🧱 Fase 1 — Infraestructura

* [x] Monorepo con Turbo
* [x] Backend NestJS
* [x] Prisma + PostgreSQL
* [ ] Cliente SOAP base
* [ ] Gestión CUIS/CUFD

### 🧾 Fase 2 — Facturación

* [ ] XML factura compra-venta
* [ ] Firma digital
* [ ] Validación SIAT
* [ ] Anulación de facturas

### ☁️ Fase 3 — Plataforma

* [ ] Dashboard web
* [ ] Multiempresa
* [ ] SDK para terceros
* [ ] Docker image oficial
* [ ] Documentación pública

---

## 📊 Estado del proyecto

<div align="center">

| Componente            | Estado |
| --------------------- | ------ |
| Backend NestJS        | 🟢     |
| Prisma/PostgreSQL     | 🟢     |
| Arquitectura Monorepo | 🟢     |
| Cliente SOAP          | 🟡     |
| CUIS/CUFD             | 🟡     |
| Emisión real SIAT     | 🔴     |
| Homologación SIN      | 🔴     |

</div>

> ⚠️ **Importante:** Supay aún **no está homologado por el SIN**. Actualmente es un proyecto **experimental y de desarrollo**.

---

## 🌱 Open Source

La idea es que cualquier desarrollador boliviano pueda:

* 📚 Aprender cómo funciona el SIAT
* 🏢 Crear su propio sistema de facturación
* 🖥️ Desplegar una versión **self-hosted**
* 🔌 Construir módulos y adaptadores
* 🤝 Contribuir al ecosistema tecnológico boliviano

---

## 🤝 Contribuir

### Crear una rama

```bash
git checkout -b feature/nueva-funcionalidad
```

### Commit

```bash
git commit -m "feat: agregar soporte CUFD"
```

### Push

```bash
git push origin feature/nueva-funcionalidad
```

Luego abre un **Pull Request** 🚀

---

## 📚 Stack visual

<div align="center">

| Backend                                          | Base de datos                                        | Infraestructura                                  |
| ------------------------------------------------ | ---------------------------------------------------- | ------------------------------------------------ |
| ![NestJS](https://skillicons.dev/icons?i=nestjs) | ![Postgres](https://skillicons.dev/icons?i=postgres) | ![Docker](https://skillicons.dev/icons?i=docker) |
| ![TypeScript](https://skillicons.dev/icons?i=ts) | ![Prisma](https://skillicons.dev/icons?i=prisma)     | ![GitHub](https://skillicons.dev/icons?i=github) |

</div>

---

## 📖 Documentación

Próximamente en:

```text
/docs
```

Incluirá:

* Arquitectura SIAT
* Flujos CUIS/CUFD
* Ejemplos XML
* Guías de homologación
* Integración con otros lenguajes

---

## 📜 Licencia

Distribuido bajo la licencia **MIT**.

```text
MIT License © 2026 Supay Contributors
```

---

## 👹 ¿Por qué “Supay”?

En la cosmovisión andina, **Supay** representa una fuerza asociada al mundo subterráneo, la transformación y el poder.

El proyecto toma ese nombre porque busca **transformar una integración fiscal compleja y oscura en una experiencia de desarrollo moderna, clara y poderosa**.

---

<div align="center">

### 🇧🇴 Hecho en Bolivia para desarrolladores bolivianos

**NestJS · TypeScript · SIAT · Open Source**

⭐ **Si el proyecto te parece útil, dale una estrella al repositorio.**

</div>
