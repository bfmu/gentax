import { useEffect, useState, type FormEvent } from 'react';
import { Link } from 'react-router-dom';
import client from '@/api/client';
import type { Driver, Taxi } from '@/api/types';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog';

const BOT_USERNAME = import.meta.env.VITE_BOT_USERNAME ?? 'gentax_bot';

export default function Drivers() {
  const [drivers, setDrivers] = useState<Driver[]>([]);
  const [taxis, setTaxis] = useState<Taxi[]>([]);
  const [name, setName] = useState('');
  const [phone, setPhone] = useState('');
  const [error, setError] = useState('');

  // Link dialog
  const [deepLink, setDeepLink] = useState('');
  const [linkDialogOpen, setLinkDialogOpen] = useState(false);

  // Assign dialog
  const [assignDriverID, setAssignDriverID] = useState('');
  const [selectedTaxiID, setSelectedTaxiID] = useState('');
  const [assignDialogOpen, setAssignDialogOpen] = useState(false);
  const [assignError, setAssignError] = useState('');

  // Unassign dialog
  const [unassignDriver, setUnassignDriver] = useState<Driver | null>(null);
  const [unassignDialogOpen, setUnassignDialogOpen] = useState(false);
  const [unassignError, setUnassignError] = useState('');

  async function load() {
    const [driversRes, taxisRes] = await Promise.all([
      client.get<Driver[]>('/drivers'),
      client.get<Taxi[]>('/taxis'),
    ]);
    setDrivers(driversRes.data ?? []);
    setTaxis((taxisRes.data ?? []).filter(t => t.active));
  }

  useEffect(() => { load(); }, []);

  async function handleCreate(e: FormEvent) {
    e.preventDefault();
    setError('');
    try {
      await client.post('/drivers', { full_name: name, phone });
      setName(''); setPhone('');
      load();
    } catch {
      setError('Error al crear conductor. Verificá los datos.');
    }
  }

  async function generateLink(driverID: string) {
    const res = await client.post<{ token: string }>(`/drivers/${driverID}/link-token`);
    setDeepLink(`https://t.me/${BOT_USERNAME}?start=${res.data.token}`);
    setLinkDialogOpen(true);
  }

  function openAssign(driverID: string) {
    setAssignDriverID(driverID);
    setSelectedTaxiID(taxis[0]?.id ?? '');
    setAssignError('');
    setAssignDialogOpen(true);
  }

  async function confirmAssign() {
    if (!selectedTaxiID) return;
    setAssignError('');
    try {
      await client.post(`/taxis/${selectedTaxiID}/assign/${assignDriverID}`);
      setAssignDialogOpen(false);
      load();
    } catch (err: unknown) {
      const e = err as { response?: { data?: { message?: string } } };
      setAssignError(e.response?.data?.message ?? 'Error al asignar taxi.');
    }
  }

  function openUnassign(driver: Driver) {
    setUnassignDriver(driver);
    setUnassignError('');
    setUnassignDialogOpen(true);
  }

  async function confirmUnassign() {
    if (!unassignDriver?.assigned_taxi) return;
    setUnassignError('');
    try {
      await client.delete(`/taxis/${unassignDriver.assigned_taxi.id}/assign/${unassignDriver.id}`);
      setUnassignDialogOpen(false);
      load();
    } catch (err: unknown) {
      const e = err as { response?: { data?: { message?: string } } };
      setUnassignError(e.response?.data?.message ?? 'Error al desasignar taxi.');
    }
  }

  return (
    <div className="min-h-screen p-8 max-w-4xl mx-auto">
      <div className="flex justify-between items-center mb-6">
        <h1 className="text-2xl font-bold">Conductores</h1>
        <Link to="/"><Button variant="outline">← Inicio</Button></Link>
      </div>

      <form onSubmit={handleCreate} className="flex gap-2 flex-wrap mb-6">
        <div className="flex flex-col gap-1">
          <Label>Nombre</Label>
          <Input placeholder="Juan Pérez" value={name} onChange={e => setName(e.target.value)} required />
        </div>
        <div className="flex flex-col gap-1">
          <Label>Teléfono</Label>
          <Input placeholder="+57 300 000 0000" value={phone} onChange={e => setPhone(e.target.value)} required />
        </div>
        <div className="flex flex-col justify-end">
          <Button type="submit">Agregar</Button>
        </div>
      </form>
      {error && <p className="text-sm text-destructive mb-4">{error}</p>}

      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Nombre</TableHead>
            <TableHead>Teléfono</TableHead>
            <TableHead>Telegram</TableHead>
            <TableHead>Taxi asignado</TableHead>
            <TableHead>Acciones</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {drivers.map(d => (
            <TableRow key={d.id}>
              <TableCell>{d.full_name}</TableCell>
              <TableCell>{d.phone}</TableCell>
              <TableCell>{d.telegram_id ? '✓ Vinculado' : 'Sin vincular'}</TableCell>
              <TableCell>{d.assigned_taxi ? d.assigned_taxi.plate : 'Sin asignar'}</TableCell>
              <TableCell className="flex gap-2 flex-wrap">
                {!d.telegram_id && (
                  <Button variant="outline" size="sm" onClick={() => generateLink(d.id)}>
                    Generar link
                  </Button>
                )}
                {!d.assigned_taxi && (
                  <Button variant="outline" size="sm" onClick={() => openAssign(d.id)}>
                    Asignar taxi
                  </Button>
                )}
                {d.assigned_taxi && (
                  <Button variant="outline" size="sm" onClick={() => openUnassign(d)}>
                    Desasignar
                  </Button>
                )}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>

      {/* Link dialog */}
      <Dialog open={linkDialogOpen} onOpenChange={setLinkDialogOpen}>
        <DialogContent>
          <DialogHeader><DialogTitle>Link de registro</DialogTitle></DialogHeader>
          <p className="text-sm text-muted-foreground mb-2">Comparte este enlace con el conductor:</p>
          <a href={deepLink} target="_blank" rel="noopener noreferrer" className="break-all text-primary underline text-sm">
            {deepLink}
          </a>
        </DialogContent>
      </Dialog>

      {/* Assign taxi dialog */}
      <Dialog open={assignDialogOpen} onOpenChange={setAssignDialogOpen}>
        <DialogContent>
          <DialogHeader><DialogTitle>Asignar taxi</DialogTitle></DialogHeader>
          {taxis.length === 0 ? (
            <p className="text-sm text-muted-foreground">No hay taxis activos disponibles. Agregá un taxi primero.</p>
          ) : (
            <div className="space-y-4">
              <div className="flex flex-col gap-1">
                <Label>Seleccioná el taxi</Label>
                <select
                  className="border rounded px-3 py-2 text-sm bg-background"
                  value={selectedTaxiID}
                  onChange={e => setSelectedTaxiID(e.target.value)}
                >
                  {taxis.map(t => (
                    <option key={t.id} value={t.id}>{t.plate} — {t.model} ({t.year})</option>
                  ))}
                </select>
              </div>
              {assignError && <p className="text-sm text-destructive">{assignError}</p>}
              <div className="flex gap-2 justify-end">
                <Button variant="outline" onClick={() => setAssignDialogOpen(false)}>Cancelar</Button>
                <Button onClick={confirmAssign}>Asignar</Button>
              </div>
            </div>
          )}
        </DialogContent>
      </Dialog>

      {/* Unassign taxi dialog */}
      <Dialog open={unassignDialogOpen} onOpenChange={setUnassignDialogOpen}>
        <DialogContent>
          <DialogHeader><DialogTitle>Desasignar taxi</DialogTitle></DialogHeader>
          <p className="text-sm text-muted-foreground">
            ¿Confirmás que querés desasignar el taxi{' '}
            <strong>{unassignDriver?.assigned_taxi?.plate}</strong> de{' '}
            <strong>{unassignDriver?.full_name}</strong>?
          </p>
          {unassignError && <p className="text-sm text-destructive mt-2">{unassignError}</p>}
          <div className="flex gap-2 justify-end mt-4">
            <Button variant="outline" onClick={() => setUnassignDialogOpen(false)}>Cancelar</Button>
            <Button variant="destructive" onClick={confirmUnassign}>Desasignar</Button>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
}
