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
import { Textarea } from '@/components/ui/textarea';
import { formatAmount } from '@/lib/format';

const STATUS_LABELS: Record<string, string> = {
  pending: 'Pendiente',
  confirmed: 'Confirmado',
  needs_evidence: 'Necesita evidencia',
  approved: 'Aprobado',
  rejected: 'Rechazado',
};

const STATUS_VARIANTS: Record<string, 'default' | 'secondary' | 'destructive' | 'outline'> = {
  pending: 'secondary',
  confirmed: 'default',
  needs_evidence: 'secondary',
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
  const [evidenceDialogOpen, setEvidenceDialogOpen] = useState(false);
  const [evidenceMessage, setEvidenceMessage] = useState('');

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

  function openRequestEvidence() {
    setEvidenceMessage('');
    setEvidenceDialogOpen(true);
  }

  async function confirmRequestEvidence() {
    await client.patch(`/expenses/${id}/request-evidence`, { message: evidenceMessage });
    setEvidenceDialogOpen(false);
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
            <span>{expense.category_name}</span>
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
              <p className="font-medium">{formatAmount(expense.amount)}</p>
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

          {expense.rejection_reason && (
            <div>
              <p className="text-muted-foreground text-sm">Motivo de rechazo</p>
              <p className="text-sm text-destructive">{expense.rejection_reason}</p>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Receipt image via proxy endpoint */}
      <Card>
        <CardHeader><CardTitle className="text-base">Recibo</CardTitle></CardHeader>
        <CardContent>
          <img
            src={`/api/expenses/${id}/receipt`}
            alt="Recibo"
            className="max-w-full rounded border"
            onError={(e) => { (e.target as HTMLImageElement).style.display = 'none'; }}
          />
        </CardContent>
      </Card>

      {(expense.status === 'confirmed' || expense.status === 'pending' || expense.status === 'needs_evidence') && (
        <div className="flex gap-3">
          {(expense.status === 'confirmed' || expense.status === 'pending') && (
            <>
              <Button onClick={approve}>Aprobar</Button>
              <Button variant="destructive" onClick={openReject}>Rechazar</Button>
            </>
          )}
          <Button variant="outline" onClick={openRequestEvidence}>Pedir más soportes</Button>
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

      <Dialog open={evidenceDialogOpen} onOpenChange={setEvidenceDialogOpen}>
        <DialogContent>
          <DialogHeader><DialogTitle>Pedir más soportes</DialogTitle></DialogHeader>
          <div className="space-y-3">
            <div>
              <Label>Mensaje para el conductor</Label>
              <Textarea
                value={evidenceMessage}
                onChange={(e: React.ChangeEvent<HTMLTextAreaElement>) => setEvidenceMessage(e.target.value)}
                placeholder="Ej: La foto del recibo está borrosa, por favor enviá otra."
                rows={3}
              />
            </div>
            <div className="flex gap-2 justify-end">
              <Button variant="outline" onClick={() => setEvidenceDialogOpen(false)}>Cancelar</Button>
              <Button onClick={confirmRequestEvidence}>Enviar solicitud</Button>
            </div>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
}
