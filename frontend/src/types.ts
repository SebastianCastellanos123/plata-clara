export type Category =
  | "comida"
  | "transporte"
  | "vivienda"
  | "servicios"
  | "salud"
  | "ocio"
  | "educacion"
  | "otros";

export interface User {
  id: string;
  email: string;
}

export interface Expense {
  id: string;
  amount: number; // pesos enteros
  category: Category;
  description: string;
  spentOn: string; // AAAA-MM-DD
}

export interface ExpenseInput {
  amount: number;
  category: Category;
  description: string;
  spentOn: string;
}

export interface CategoryTotal {
  category: Category;
  total: number;
  count: number;
}

export interface Summary {
  total: number;
  byCategory: CategoryTotal[];
}
