# driver-assignment-panel Specification

## Purpose

Permite al owner ver y gestionar asignaciones taxi-conductor desde el panel web.
Cubre: visualización de asignación actual, asignar taxi a conductor, desasignar taxi.

## Requirements

### Requirement: Driver List Includes Assignment

`GET /drivers` MUST return an `assigned_taxi` field for each driver.
The field MUST be `{ id: string, plate: string }` when an active assignment exists.
The field MUST be `null` when no active assignment exists.

#### Scenario: Driver with active assignment

- GIVEN a driver has an active taxi assignment in `driver_assignments`
- WHEN the owner calls `GET /drivers`
- THEN the driver entry includes `assigned_taxi: { id, plate }` with the assigned taxi's data

#### Scenario: Driver without assignment

- GIVEN a driver has no active taxi assignment
- WHEN the owner calls `GET /drivers`
- THEN the driver entry includes `assigned_taxi: null`

#### Scenario: Multiple drivers mixed

- GIVEN some drivers have assignments and others do not
- WHEN the owner calls `GET /drivers`
- THEN each driver entry reflects its own assignment status independently

---

### Requirement: Panel Shows Assignment Column

The Drivers table in the web panel MUST display a "Taxi asignado" column.
The column MUST show the taxi plate when a driver has an active assignment.
The column MUST show "Sin asignar" when `assigned_taxi` is null.

#### Scenario: Assigned driver in table

- GIVEN a driver has `assigned_taxi: { plate: "ABC-123" }`
- WHEN the owner views the Drivers page
- THEN the row shows "ABC-123" in the "Taxi asignado" column

#### Scenario: Unassigned driver in table

- GIVEN a driver has `assigned_taxi: null`
- WHEN the owner views the Drivers page
- THEN the row shows "Sin asignar" in the "Taxi asignado" column

---

### Requirement: Assign Taxi from Panel

The panel MUST provide an "Asignar taxi" action for drivers without an active assignment.
The action MUST call `POST /taxis/{taxiID}/assign/{driverID}`.
On success, the table MUST refresh showing the new assignment.

#### Scenario: Successful assignment

- GIVEN a driver has no active assignment and active taxis exist
- WHEN the owner selects a taxi and confirms assignment
- THEN the driver row shows the assigned taxi's plate
- AND the "Asignar taxi" action is no longer shown for that driver

#### Scenario: No active taxis available

- GIVEN no active taxis exist for the owner
- WHEN the owner opens the assign dialog
- THEN a message informs there are no active taxis available

#### Scenario: Driver already assigned — assign hidden

- GIVEN a driver has an active assignment
- WHEN the owner views the Drivers table
- THEN the "Asignar taxi" button MUST NOT be shown for that driver

---

### Requirement: Unassign Taxi from Panel

The panel MUST provide a "Desasignar" action for drivers with an active assignment.
The action MUST call `DELETE /taxis/{taxiID}/assign/{driverID}`.
On success, the table MUST refresh showing "Sin asignar" for that driver.

#### Scenario: Successful unassignment

- GIVEN a driver has an active assignment
- WHEN the owner confirms unassignment
- THEN the driver row shows "Sin asignar"
- AND the "Desasignar" button is no longer shown

#### Scenario: Driver without assignment — unassign hidden

- GIVEN a driver has no active assignment
- WHEN the owner views the Drivers table
- THEN the "Desasignar" button MUST NOT be shown for that driver
