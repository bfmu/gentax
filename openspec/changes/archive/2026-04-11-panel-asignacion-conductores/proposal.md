# Proposal: Panel Asignación Conductores

## Intent

El owner no puede ver ni gestionar asignaciones taxi-conductor desde el panel web. La tabla de conductores no muestra si tienen taxi asignado, no existe botón de desasignación, y `GET /drivers` no devuelve info de asignación. Esto bloquea al conductor cuando intenta `/gasto` en el bot ("No tenés taxis asignados").

## Scope

### In Scope
- Enriquecer `GET /drivers` para incluir `assigned_taxi: { id, plate } | null` en cada conductor
- Agregar columna "Taxi asignado" a la tabla Drivers (placa o "Sin asignar")
- Agregar botón "Desasignar" cuando el conductor tiene taxi activo
- Incorporar y commitear botón "Asignar taxi" (ya existe en Drivers.tsx sin commitear)
- Limpiar `cmd/web/main.go` líneas 55–92 (texto basura)

### Out of Scope
- Creación/eliminación de taxis
- Mejoras al bot de Telegram (change separado: `ux-bot-conductor`)
- Historial de asignaciones

## Capabilities

### New Capabilities
- `driver-assignment-panel`: Gestión completa de asignaciones taxi-conductor desde el panel web

### Modified Capabilities
- Ninguna

## Approach

1. Backend: nuevo endpoint `GET /drivers` enriquecido — join con `driver_assignments` activos para incluir `assigned_taxi`. Alternativa: endpoint dedicado `GET /drivers/{id}/assignment`. **Preferido**: enriquecer respuesta existente para evitar N+1 en el frontend.
2. Frontend: actualizar tipo `Driver` en `api/types.ts` con campo `assigned_taxi`. Agregar columna + botón desasignar en `Drivers.tsx`.
3. No hay migración de DB necesaria — la tabla `driver_assignments` ya existe.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/httpapi/handlers/drivers.go` | Modified | List handler retorna assigned_taxi |
| `internal/driver/driver.go` | Modified | Driver struct + nuevo DriverWithAssignment view |
| `internal/driver/service.go` | Modified | List devuelve vista enriquecida |
| `internal/driver/repository.go` | Modified | Query con join a driver_assignments |
| `web/src/api/types.ts` | Modified | Driver type agrega assigned_taxi |
| `web/src/pages/Drivers.tsx` | Modified | Columna + botón desasignar |
| `cmd/web/main.go` | Modified | Eliminar líneas 55–92 |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Query con join rompe tests existentes de driver.List | Med | Actualizar mocks y snapshots |
| Desasignación sin confirmación del usuario | Low | Agregar confirmación en dialog |

## Rollback Plan

Revertir los commits del change. No hay migración de DB — rollback es solo código.

## Dependencies

- Ninguna externa. El backend de asignación ya existe.

## Success Criteria

- [ ] `GET /drivers` incluye `assigned_taxi` con placa o null
- [ ] Tabla Drivers muestra columna "Taxi asignado"
- [ ] Botón "Asignar taxi" funciona (ya implementado, commitear)
- [ ] Botón "Desasignar" aparece solo cuando hay asignación activa y funciona
- [ ] `cmd/web/main.go` sin texto basura
- [ ] Tests de handler y service pasan con los cambios
