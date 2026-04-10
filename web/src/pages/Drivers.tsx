import { useEffect, useState, type FormEvent } from 'react';
import { Link } from 'react-router-dom';
import client from '@/api/client';
import type { Driver } from '@/api/types';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog';

const BOT_USERNAME = import.meta.env.VITE_BOT_USERNAME ?? 'gentax_bot';

export default function Drivers() {
  const [drivers, setDrivers] = useState<Driver[]>([]);
  const [name, setName] = useState('');
  const [phone, setPhone] = useState('');
  const [deepLink, setDeepLink] = useState('');
  const [dialogOpen, setDialogOpen] = useState(false);
  const [error, setError] = useState('');

  async function load() {
    const res = await client.get<Driver[]>('/drivers');
    setDrivers(res.data ?? []);
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
    setDialogOpen(true);
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
            <TableHead></TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {drivers.map(d => (
            <TableRow key={d.id}>
              <TableCell>{d.full_name}</TableCell>
              <TableCell>{d.phone}</TableCell>
              <TableCell>{d.telegram_id ? '✓ Vinculado' : 'Sin vincular'}</TableCell>
              <TableCell>
                {!d.telegram_id && (
                  <Button variant="outline" size="sm" onClick={() => generateLink(d.id)}>
                    Generar link
                  </Button>
                )}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent>
          <DialogHeader><DialogTitle>Link de registro</DialogTitle></DialogHeader>
          <p className="text-sm text-muted-foreground mb-2">Comparte este enlace con el conductor:</p>
          <a href={deepLink} target="_blank" rel="noopener noreferrer" className="break-all text-primary underline text-sm">
            {deepLink}
          </a>
        </DialogContent>
      </Dialog>
    </div>
  );
}
