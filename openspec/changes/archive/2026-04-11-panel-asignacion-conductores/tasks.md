# Tasks: Panel Asignación Conductores

> Strict TDD: cada capa sigue RED → GREEN. Tests primero, implementación después.

## Phase 1: Domain — tipos e interfaces

- [x] 1.1 `internal/driver/driver.go` — agregar `AssignedTaxiView` struct (`ID uuid.UUID`, `Plate string`, json tags)
- [x] 1.2 `internal/driver/driver.go` — agregar `DriverWithAssignment` struct (embed `*Driver` + `AssignedTaxi *AssignedTaxiView`)
- [x] 1.3 `internal/driver/driver.go` — agregar `ListWithAssignment(ctx, ownerID) ([]*DriverWithAssignment, error)` a la interfaz `Repository`
- [x] 1.4 `internal/driver/driver.go` — agregar `ListWithAssignment(ctx, ownerID) ([]*DriverWithAssignment, error)` a la interfaz `Service`

## Phase 2: Repository (TDD)

- [x] 2.1 `internal/driver/mock_repository.go` — agregar método `ListWithAssignment` al mock (necesario para que compilen los tests de service)
- [x] 2.2 `internal/driver/repository_integration_test.go` — escribir tests RED: `TestListWithAssignment_WithActiveAssignment` y `TestListWithAssignment_NoAssignment`
- [x] 2.3 `internal/driver/repository.go` — implementar `ListWithAssignment` con LEFT JOIN a `driver_taxi_assignments` + `taxis` (hacer pasar los tests del 2.2)

## Phase 3: Service (TDD)

- [x] 3.1 `internal/driver/service_test.go` — escribir tests RED: `TestService_ListWithAssignment_Delegates` verifica que delega al repo
- [x] 3.2 `internal/driver/service.go` — implementar `ListWithAssignment` delegando al repo (hacer pasar 3.1)

## Phase 4: Handler (TDD)

- [x] 4.1 `internal/httpapi/handlers/mock_services_test.go` — agregar `ListWithAssignment` a `mockDriverService`
- [x] 4.2 `internal/httpapi/handlers/drivers_test.go` — escribir tests RED: `TestDriverHandler_List_IncludesAssignment` (con y sin taxi asignado)
- [x] 4.3 `internal/httpapi/handlers/drivers.go` — actualizar `List` handler para llamar `driverSvc.ListWithAssignment` en lugar de `driverSvc.List` (hacer pasar 4.2)

## Phase 5: Frontend

- [x] 5.1 `web/src/api/types.ts` — agregar `interface AssignedTaxi { id: string; plate: string }` y campo `assigned_taxi: AssignedTaxi | null` al tipo `Driver`
- [x] 5.2 `web/src/pages/Drivers.tsx` — agregar columna "Taxi asignado" (muestra `assigned_taxi.plate` o "Sin asignar")
- [x] 5.3 `web/src/pages/Drivers.tsx` — mostrar botón "Asignar taxi" solo cuando `!d.assigned_taxi`; agregar botón "Desasignar" cuando `d.assigned_taxi`, llamando `DELETE /taxis/{d.assigned_taxi.id}/assign/{d.id}`

## Phase 6: Cleanup

- [x] 6.1 `cmd/web/main.go` — eliminar líneas 55–92 (texto basura que quedó accidentalmente)
