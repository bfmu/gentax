import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import client from '@/api/client';
import type { Expense } from '@/api/types';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog';

type Tab = 'confirmed' | 'needs_evidence' | 'approved' | 'rejected' | 'all';

const TAB_LABELS: Record<Tab, string> = {
  confirmed: 'Por aprobar',
  needs_evidence: 'Esperando soporte',
  approved: 'Aprobados',
  rejected: 'Rechazados',
  all: 'Todos',
};

const STATUS_LABELS: Record<string, string> = {
  pending: 'Pendiente',
  confirmed: 'Por aprobar',
  needs_evidence: 'Esperando soporte',
  approved: 'Aprobado',
  rejected: 'Rechazado',
};

const STATUS_VARIANTS: Record<string, 'default' | 'secondary' | 'destructive' | 'outline'> = {
  pending: 'secondary',
  confirmed: 'default',
  needs_evidence: 'secondary',
  approved: 'outline',
  rejected: 'destructive',
};

export default function Expenses() {
  const [expenses, setExpenses] = useState<Expense[]>([]);
  const [tab, setTab] = useState<Tab>('confirmed');
  const [rejectID, setRejectID] = useState('');
  const [reason, setReason] = useState('');
  const [dialogOpen, setDialogOpen] = useState(false);

  async function load(status: Tab) {
    const query = status === 'all' ? '' : `?status=${status}`;
    const res = await client.get<Expense[]>(`/expenses${query}`);
    setExpenses(res.data ?? []);
  }

  useEffect(() => { load(tab); }, [tab]);

  async function approve(id: string) {
    await client.patch(`/expenses/${id}/approve`);
    load(tab);
  }

  function openReject(id: string) {
    setRejectID(id);
    setReason('');
    setDialogOpen(true);
  }

  async function confirmReject() {
    await client.patch(`/expenses/${rejectID}/reject`, { reason });
    setDialogOpen(false);
    load(tab);
  }

  return (
    <div className="min-h-screen p-8 max-w-5xl mx-auto">
      <div className="flex justify-between items-center mb-6">
        <h1 className="text-2xl font-bold">Gestión de Gastos</h1>
        <Link to="/"><Button variant="outline">← Inicio</Button></Link>
      </div>

      {/* Status tabs */}
      <div className="flex gap-2 mb-4 flex-wrap">
        {(Object.keys(TAB_LABELS) as Tab[]).map(t => (
          <Button
            key={t}
            size="sm"
            variant={tab === t ? 'default' : 'outline'}
            onClick={() => setTab(t)}
          >
            {TAB_LABELS[t]}
          </Button>
        ))}
      </div>

      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Conductor</TableHead>
            <TableHead>Taxi</TableHead>
            <TableHead>Categoría</TableHead>
            <TableHead>Monto (COP)</TableHead>
            <TableHead>Estado</TableHead>
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
              <TableCell>
                <Badge variant={STATUS_VARIANTS[e.status] ?? 'default'}>
                  {STATUS_LABELS[e.status] ?? e.status}
                </Badge>
              </TableCell>
              <TableCell className="flex gap-2">
                <Link to={`/expenses/${e.id}`}>
                  <Button size="sm" variant="outline">Ver detalle</Button>
                </Link>
                {e.status === 'confirmed' && (
                  <>
                    <Button size="sm" onClick={() => approve(e.id)}>Aprobar</Button>
                    <Button size="sm" variant="destructive" onClick={() => openReject(e.id)}>Rechazar</Button>
                  </>
                )}
              </TableCell>
            </TableRow>
          ))}
          {expenses.length === 0 && (
            <TableRow><TableCell colSpan={6} className="text-center text-muted-foreground">Sin gastos en esta categoría</TableCell></TableRow>
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
