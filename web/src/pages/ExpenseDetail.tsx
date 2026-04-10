import { useEffect, useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import client from '@/api/client';
import type { Expense } from '@/api/types';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';

const STATUS_LABELS: Record<string, string> = {
  pending: 'Pendiente',
  confirmed: 'Confirmado',
  approved: 'Aprobado',
  rejected: 'Rechazado',
};

const STATUS_VARIANTS: Record<string, 'default' | 'secondary' | 'destructive' | 'outline'> = {
  pending: 'secondary',
  confirmed: 'default',
  approved: 'default',
  rejected: 'destructive',
};

export default function ExpenseDetail() {
  const { id } = useParams<{ id: string }>();
  const [expense, setExpense] = useState<Expense | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [dialogOpen, setDialogOpen] = useState(false);
  const [reason, setReason] = useState('');

  async function load() {
    try {
      setLoading(true);
      setError('');
      const res = await client.get<Expense>(`/expenses/${id}`);
      setExpense(res.data);
    } catch {
      setError('No se pudo cargar el gasto.');
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => { load(); }, [id]);

  async function approve() {
    await client.patch(`/expenses/${id}/approve`);
    load();
  }

  function openReject() {
    setReason('');
    setDialogOpen(true);
  }

  async function confirmReject() {
    await client.patch(`/expenses/${id}/reject`, { reason });
    setDialogOpen(false);
    load();
  }

  if (loading) {
    return (
      <div className="min-h-screen p-8 max-w-3xl mx-auto">
        <p className="text-muted-foreground">Cargando...</p>
      </div>
    );
  }

  if (error || !expense) {
    return (
      <div className="min-h-screen p-8 max-w-3xl mx-auto">
        <p className="text-destructive">{error || 'Gasto no encontrado.'}</p>
        <Link to="/expenses"><Button variant="outline" className="mt-4">← Volver</Button></Link>
      </div>
    );
  }

  return (
    <div className="min-h-screen p-8 max-w-3xl mx-auto space-y-6">
      <div className="flex justify-between items-center">
        <h1 className="text-2xl font-bold">Detalle del Gasto</h1>
        <Link to="/expenses"><Button variant="outline">← Volver</Button></Link>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center justify-between">
            <span>{expense.category}</span>
            <Badge variant={STATUS_VARIANTS[expense.status] ?? 'default'}>
              {STATUS_LABELS[expense.status] ?? expense.status}
            </Badge>
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="grid grid-cols-2 gap-4 text-sm">
            <div>
              <p className="text-muted-foreground">Conductor</p>
              <p className="font-medium">{expense.driver_name}</p>
            </div>
            <div>
              <p className="text-muted-foreground">Placa taxi</p>
              <p className="font-medium">{expense.taxi_plate}</p>
            </div>
            <div>
              <p className="text-muted-foreground">Monto (COP)</p>
              <p className="font-medium">{Number(expense.amount).toLocaleString('es-CO')}</p>
            </div>
            <div>
              <p className="text-muted-foreground">Fecha</p>
              <p className="font-medium">{new Date(expense.created_at).toLocaleDateString('es-CO')}</p>
            </div>
          </div>

          {expense.notes && (
            <div>
              <p className="text-muted-foreground text-sm">Descripción</p>
              <p className="text-sm">{expense.notes}</p>
            </div>
          )}

          {expense.reject_reason && (
            <div>
              <p className="text-muted-foreground text-sm">Motivo de rechazo</p>
              <p className="text-sm text-destructive">{expense.reject_reason}</p>
            </div>
          )}
        </CardContent>
      </Card>

      {expense.receipt_image_url && (
        <Card>
          <CardHeader><CardTitle className="text-base">Recibo</CardTitle></CardHeader>
          <CardContent>
            <img
              src={expense.receipt_image_url}
              alt="Recibo"
              className="max-w-full rounded border"
            />
          </CardContent>
        </Card>
      )}

      {expense.ocr_text && (
        <Card>
          <CardHeader><CardTitle className="text-base">Texto OCR</CardTitle></CardHeader>
          <CardContent>
            <pre className="text-xs whitespace-pre-wrap bg-muted p-3 rounded">{expense.ocr_text}</pre>
          </CardContent>
        </Card>
      )}

      {expense.status === 'confirmed' && (
        <div className="flex gap-3">
          <Button onClick={approve}>Aprobar</Button>
          <Button variant="destructive" onClick={openReject}>Rechazar</Button>
        </div>
      )}

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent>
          <DialogHeader><DialogTitle>Rechazar gasto</DialogTitle></DialogHeader>
          <div className="space-y-3">
            <div>
              <Label>Motivo (opcional)</Label>
              <Input
                value={reason}
                onChange={(e: React.ChangeEvent<HTMLInputElement>) => setReason(e.target.value)}
                placeholder="Ej: recibo ilegible"
              />
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
