import { redirect } from '@sveltejs/kit';

export const load = () => {
  const today = new Date().toISOString().split('T')[0];
  redirect(302, `/day/${today}`);
};
