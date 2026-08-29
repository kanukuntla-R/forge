import { prisma } from "@/lib/prisma"

export default async function UsersPage() {
  const users = await prisma.user.findMany()

  return (
    <main>
      <h1>Users</h1>
      <ul>
        {users.map((user) => (
          <li key={user.id}>{user.name ?? user.email}</li>
        ))}
      </ul>
    </main>
  )
}
