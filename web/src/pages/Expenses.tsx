import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import client from '@/api/client';
import type { Expense } from '@/api/types';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog';

export default function Expenses() {
  const [expenses, setExpenses] = useState<Expense[]>([]);
  const [rejectID, setRejectID] = useState('');
  const [reason, setReason] = useState('');
  const [dialogOpen, setDialogOpen] = useState(false);

  async function load() {
    const res = await client.get<Expense[]>('/expenses?status=confirmed');
    setExpenses(res.data ?? []);
  }

  useEffect(() => { load(); }, []);

  async function approve(id: string) {
    await client.patch(`/expenses/${id}/approve`);
    load();
  }

  function openReject(id: string) {
    setRejectID(id);
    setReason('');
    setDialogOpen(true);
  }

  async function confirmReject() {
    await client.patch(`/expenses/${rejectID}/reject`, { reason });
    setDialogOpen(false);
    load();
  }

  return (
    <div className="min-h-screen p-8 max-w-5xl mx-auto">
      <div className="flex justify-between items-center mb-6">
        <h1 className="text-2xl font-bold">Gastos Pendientes</h1>
        <Link to="/"><Button variant="outline">← Inicio</Button></Link>
      </div>

      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Conductor</TableHead>
            <TableHead>Taxi</TableHead>
            <TableHead>Categoría</TableHead>
            <TableHead>Monto (COP)</TableHead>
            <TableHead></TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {expenses.map(e => (
            <TableRow key={e.id}>
              <TableCell>{e.driver_name}</TableCell>
              <TableCell>{e.taxi_plate}</TableCell>
              <TableCell>{e.category_name}</TableCell>
              <TableCell>{Number(e.amount).toLocaleString('es-CO')}</TableCell>
              <TableCell className="flex gap-2">
                <Button size="sm" onClick={() => approve(e.id)}>Aprobar</Button>
                <Button size="sm" variant="destructive" onClick={() => openReject(e.id)}>Rechazar</Button>
              </TableCell>
            </TableRow>
          ))}
          {expenses.length === 0 && (
            <TableRow><TableCell colSpan={5} className="text-center text-muted-foreground">Sin gastos pendientes</TableCell></TableRow>
          )}
        </TableBody>
      </Table>

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent>
          <DialogHeader><DialogTitle>Rechazar gasto</DialogTitle></DialogHeader>
          <div className="space-y-3">
            <div>
              <Label>Motivo (opcional)</Label>
              <Input value={reason} onChange={e => setReason(e.target.value)} placeholder="Ej: recibo ilegible" />
            </div>
            <div className="flex gap-2 justify-end">
              <Button variant="outline" onClick={() => setDialogOpen(false)}>Cancelar</Button>
              <Button variant="destructive" onClick={confirmReject}>Confirmar rechazo</Button>
            </div>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
}
