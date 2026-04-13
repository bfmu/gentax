export interface Owner {
  id: string;
  name: string;
  email: string;
  created_at: string;
}

export interface Taxi {
  id: string;
  owner_id: string;
  plate: string;
  model: string;
  year: number;
  active: boolean;
  created_at: string;
}

export interface AssignedTaxi {
  id: string;
  plate: string;
}

export interface Driver {
  id: string;
  owner_id: string;
  full_name: string;
  phone: string;
  telegram_id: number | null;
  active: boolean;
  created_at: string;
  assigned_taxi: AssignedTaxi | null;
}

export interface Expense {
  id: string;
  owner_id: string;
  driver_id: string;
  taxi_id: string;
  driver_name: string;
  taxi_plate: string;
  category_name: string;
  amount: string | null;
  status: 'pending' | 'confirmed' | 'needs_evidence' | 'approved' | 'rejected';
  notes: string;
  rejection_reason: string;
  receipt_image_url: string | null;
  ocr_raw?: string | null;
  created_at: string;
}

export interface Category {
  id: string;
  owner_id: string;
  name: string;
  created_at: string;
}

export interface Report {
  taxi_plate?: string;
  driver_name?: string;
  total: string;
}
