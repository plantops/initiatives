# Public deployment gate

Do not expose the local MVP directly to the Internet.

Before a public URL:

1. PostgreSQL on a managed or backed-up host.
2. HTTPS through a reverse proxy/load balancer.
3. `COOKIE_SECURE=1`.
4. Replace or integrate local passwords with company SSO/AD if available.
5. Rate-limit login and sensitive POST routes.
6. Add account lockout/recovery administration.
7. Move attachments to controlled object/file storage; define retention and malware scanning policy.
8. Add immutable audit events for authentication, role changes, master-data changes and downloads.
9. Define backup/restore test and retention.
10. Review CSRF/body-size/upload/content-disposition handling under the public threat model.
11. Add PostgreSQL migrations and integration tests before cutover.

Scale only after these gates; no microservices are needed for the expected workflow size.
