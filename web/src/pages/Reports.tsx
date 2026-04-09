import { useState, type FormEvent } from 'react';
import { Link } from 'react-router-dom';
import client from '@/api/client';
import type { Report } from '@/api/types';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';

export default function Reports() {
  const [dateFrom, setDateFrom] = useState('');
  const [dateTo, setDateTo] = useState('');
  const [taxiReports, setTaxiReports] = useState<Report[]>([]);
  const [driverReports, setDriverReports] = useState<Report[]>([]);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    const [taxiRes, driverRes] = await Promise.all([
      client.get<{ reports: Report[] }>(`/reports/taxis?date_from=${dateFrom}&date_to=${dateTo}`),
      client.get<{ reports: Report[] }>(`/reports/drivers?date_from=${dateFrom}&date_to=${dateTo}`),
    ]);
    setTaxiReports(taxiRes.data.reports ?? []);
    setDriverReports(driverRes.data.reports ?? []);
  }

  return (
    <div className="min-h-screen p-8 max-w-4xl mx-auto">
      <div className="flex justify-between items-center mb-6">
        <h1 className="text-2xl font-bold">Reportes</h1>
        <Link to="/"><Button variant="outline">← Inicio</Button></Link>
      </div>

      <form onSubmit={handleSubmit} className="flex gap-2 flex-wrap mb-8">
        <div className="flex flex-col gap-1">
          <Label>Desde</Label>
          <Input type="date" value={dateFrom} onChange={e => setDateFrom(e.target.value)} required />
        </div>
        <div className="flex flex-col gap-1">
          <Label>Hasta</Label>
          <Input type="date" value={dateTo} onChange={e => setDateTo(e.target.value)} required />
        </div>
        <div className="flex flex-col justify-end">
          <Button type="submit">Consultar</Button>
        </div>
      </form>

      {taxiReports.length > 0 && (
        <div className="mb-6">
          <h2 className="text-lg font-semibold mb-2">Por Taxi</h2>
          <Table>
            <TableHeader>
              <TableRow><TableHead>Placa</TableHead><TableHead>Total (COP)</TableHead></TableRow>
            </TableHeader>
            <TableBody>
              {taxiReports.map((r, i) => (
                <TableRow key={i}>
                  <TableCell>{r.taxi_plate}</TableCell>
                  <TableCell>{Number(r.total).toLocaleString('es-CO')}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}

      {driverReports.length > 0 && (
        <div>
          <h2 className="text-lg font-semibold mb-2">Por Conductor</h2>
          <Table>
            <TableHeader>
              <TableRow><TableHead>Conductor</TableHead><TableHead>Total (COP)</TableHead></TableRow>
            </TableHeader>
            <TableBody>
              {driverReports.map((r, i) => (
                <TableRow key={i}>
                  <TableCell>{r.driver_name}</TableCell>
                  <TableCell>{Number(r.total).toLocaleString('es-CO')}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}
    </div>
  );
}
