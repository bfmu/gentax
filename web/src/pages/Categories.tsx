import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import client from '@/api/client';
import type { Category } from '@/api/types';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';

export default function Categories() {
  const [categories, setCategories] = useState<Category[]>([]);
  const [newName, setNewName] = useState('');
  const [error, setError] = useState('');

  async function load() {
    const res = await client.get<Category[]>('/categories');
    setCategories(res.data ?? []);
  }

  useEffect(() => {
    load();
  }, []);

  async function create() {
    if (!newName.trim()) return;
    try {
      setError('');
      await client.post('/categories', { name: newName.trim() });
      setNewName('');
      load();
    } catch (e: unknown) {
      const err = e as { response?: { data?: { error?: string } } };
      setError(err.response?.data?.error || 'Error al crear categoría');
    }
  }

  async function remove(id: string) {
    try {
      setError('');
      await client.delete(`/categories/${id}`);
      load();
    } catch (e: unknown) {
      const err = e as { response?: { data?: { error?: string } } };
      setError(err.response?.data?.error || 'No se puede eliminar: la categoría tiene gastos asociados');
    }
  }

  return (
    <div className="min-h-screen p-8 max-w-2xl mx-auto">
      <div className="flex justify-between items-center mb-6">
        <h1 className="text-2xl font-bold">Categorías de Gastos</h1>
        <Link to="/"><Button variant="outline">← Inicio</Button></Link>
      </div>

      <div className="flex gap-2 mb-6">
        <Input
          value={newName}
          onChange={e => setNewName(e.target.value)}
          onKeyDown={e => e.key === 'Enter' && create()}
          placeholder="Nueva categoría..."
          className="max-w-xs"
        />
        <Button onClick={create}>Agregar</Button>
      </div>

      {error && <p className="text-destructive text-sm mb-4">{error}</p>}

      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Nombre</TableHead>
            <TableHead></TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {categories.map(cat => (
            <TableRow key={cat.id}>
              <TableCell>{cat.name}</TableCell>
              <TableCell className="text-right">
                <Button size="sm" variant="destructive" onClick={() => remove(cat.id)}>Eliminar</Button>
              </TableCell>
            </TableRow>
          ))}
          {categories.length === 0 && (
            <TableRow>
              <TableCell colSpan={2} className="text-center text-muted-foreground">Sin categorías</TableCell>
            </TableRow>
          )}
        </TableBody>
      </Table>
    </div>
  );
}
