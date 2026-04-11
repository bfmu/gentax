# Design: Panel Asignación Conductores

## Technical Approach

Agregar un nuevo método `ListWithAssignment` en los tres capas (Repository → Service → Handler) que reemplaza la llamada existente `List` en el endpoint `GET /drivers`. El repositorio hace un LEFT JOIN con `driver_taxi_assignments` y `taxis` en una sola query — sin N+1. El frontend recibe `assigned_taxi: {id, plate} | null` y renderiza columna + botones condicionalmente.

## Architecture Decisions

| Option | Tradeoff | Decision |
|--------|----------|----------|
| Modificar `Driver` struct con campo `AssignedTaxi` | Mezcla dominio con vista | ❌ Rechazado |
| Nuevo endpoint `GET /drivers/{id}/assignment` | N requests desde frontend | ❌ Rechazado |
| Nueva query LEFT JOIN + nuevo tipo `DriverWithAssignment` | Query extra en interfaz, pero limpio | ✅ Elegido |
| Cambiar return type de `List` existente | Rompe todos los callers del Service | ❌ Rechazado |

**Decisión**: Nuevo método `ListWithAssignment` coexiste con `List`. El handler `drivers.go` usa `ListWithAssignment`; el bot sigue usando el repo directamente y no se ve afectado.

## Data Flow

```
GET /drivers
    │
    ▼
DriverHandler.List
    │ calls
    ▼
driver.Service.ListWithAssignment(ctx, ownerID)
    │ calls
    ▼
driver.Repository.ListWithAssignment(ctx, ownerID)
    │ SQL LEFT JOIN
    ▼
drivers + driver_taxi_assignments + taxis
    │
    ▼
[]*DriverWithAssignment  ──→  JSON response
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/driver/driver.go` | Modify | Agregar `DriverWithAssignment`, `AssignedTaxiView`; extender interfaces |
| `internal/driver/repository.go` | Modify | Agregar `ListWithAssignment` con LEFT JOIN |
| `internal/driver/service.go` | Modify | Agregar `ListWithAssignment` delegando al repo |
| `internal/driver/mock_repository.go` | Modify | Agregar método mock `ListWithAssignment` |
| `internal/httpapi/handlers/drivers.go` | Modify | Handler `List` llama `ListWithAssignment` |
| `internal/httpapi/handlers/mock_services_test.go` | Modify | Agregar `ListWithAssignment` a `mockDriverService` |
| `internal/httpapi/handlers/drivers_test.go` | Modify | Actualizar test de List con `DriverWithAssignment` |
| `web/src/api/types.ts` | Modify | Agregar `assigned_taxi` al tipo `Driver` |
| `web/src/pages/Drivers.tsx` | Modify | Columna "Taxi asignado" + botón "Desasignar" |
| `cmd/web/main.go` | Modify | Eliminar líneas 55–92 (basura) |

## Interfaces / Contracts

```go
// driver/driver.go — new types
type AssignedTaxiView struct {
    ID    uuid.UUID `json:"id"`
    Plate string    `json:"plate"`
}

type DriverWithAssignment struct {
    *Driver
    AssignedTaxi *AssignedTaxiView `json:"assigned_taxi"`
}

// Repository — new method
ListWithAssignment(ctx context.Context, ownerID uuid.UUID) ([]*DriverWithAssignment, error)

// Service — new method  
ListWithAssignment(ctx context.Context, ownerID uuid.UUID) ([]*DriverWithAssignment, error)
```

```sql
-- repository.go query
SELECT d.id, d.owner_id, d.telegram_id, d.full_name, d.phone, d.active,
       d.link_token, d.link_token_expires_at, d.link_token_used, d.created_at,
       t.id, t.plate
FROM drivers d
LEFT JOIN driver_taxi_assignments dta
    ON dta.driver_id = d.id AND dta.unassigned_at IS NULL
LEFT JOIN taxis t ON t.id = dta.taxi_id
WHERE d.owner_id = $1
ORDER BY d.created_at DESC
```

```typescript
// types.ts — updated Driver
interface AssignedTaxi { id: string; plate: string; }
interface Driver {
  // ... existing fields ...
  assigned_taxi: AssignedTaxi | null;
}
```

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit (handler) | `List` retorna `DriverWithAssignment` con/sin asignación | testify/mock — `mockDriverService.ListWithAssignment` |
| Unit (service) | `ListWithAssignment` delega al repo | testify/mock — `mockRepository.ListWithAssignment` |
| Integration (repo) | Query LEFT JOIN retorna placa correcta, null cuando no hay asignación | testcontainers + fixtures |

## Migration / Rollout

No migration required. La tabla `driver_taxi_assignments` y `taxis` ya existen.

## Open Questions

- Ninguna.
