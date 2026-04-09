import { useEffect, useState, type FormEvent } from 'react';
import { Link } from 'react-router-dom';
import client from '@/api/client';
import type { Taxi } from '@/api/types';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Badge } from '@/components/ui/badge';

export default function Taxis() {
  const [taxis, setTaxis] = useState<Taxi[]>([]);
  const [plate, setPlate] = useState('');
  const [model, setModel] = useState('');
  const [year, setYear] = useState('');
  const [error, setError] = useState('');

  async function load() {
    const res = await client.get<{ taxis: Taxi[] }>('/taxis');
    setTaxis(res.data.taxis ?? []);
  }

  useEffect(() => { load(); }, []);

  async function handleCreate(e: FormEvent) {
    e.preventDefault();
    setError('');
    try {
      await client.post('/taxis', { plate, model, year: parseInt(year) });
      setPlate(''); setModel(''); setYear('');
      load();
    } catch (err: unknown) {
      const e = err as { response?: { data?: { code?: string } } };
      if (e.response?.data?.code === 'duplicate_plate') {
        setError('La placa ya está registrada.');
      } else {
        setError('Error al crear taxi.');
      }
    }
  }

  return (
    <div className="min-h-screen p-8 max-w-4xl mx-auto">
      <div className="flex justify-between items-center mb-6">
        <h1 className="text-2xl font-bold">Taxis</h1>
        <Link to="/"><Button variant="outline">← Inicio</Button></Link>
      </div>

      <form onSubmit={handleCreate} className="flex gap-2 flex-wrap mb-6">
        <div className="flex flex-col gap-1">
          <Label>Placa</Label>
          <Input placeholder="ABC123" value={plate} onChange={e => setPlate(e.target.value)} required />
        </div>
        <div className="flex flex-col gap-1">
          <Label>Modelo</Label>
          <Input placeholder="Toyota Corolla" value={model} onChange={e => setModel(e.target.value)} required />
        </div>
        <div className="flex flex-col gap-1">
          <Label>Año</Label>
          <Input type="number" placeholder="2022" value={year} onChange={e => setYear(e.target.value)} required />
        </div>
        <div className="flex flex-col justify-end">
          <Button type="submit">Agregar</Button>
        </div>
      </form>
      {error && <p className="text-sm text-destructive mb-4">{error}</p>}

      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Placa</TableHead>
            <TableHead>Modelo</TableHead>
            <TableHead>Año</TableHead>
            <TableHead>Estado</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {taxis.map(t => (
            <TableRow key={t.id}>
              <TableCell>{t.plate}</TableCell>
              <TableCell>{t.model}</TableCell>
              <TableCell>{t.year}</TableCell>
              <TableCell><Badge variant={t.active ? 'default' : 'secondary'}>{t.active ? 'Activo' : 'Inactivo'}</Badge></TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}
