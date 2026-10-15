// One function per page or form, to be reused by the load test (Tarea 12).
import http from 'k6/http';

export const BASE_URL = __ENV.BASE_URL || 'http://localhost:8082';

export const SUNGLASSES = 'OLJCESPC7Z'; // category "accessories": has an ad

export const ADDRESS = {
  email: 'someone@example.com',
  street_address: '1600 Amphitheatre Parkway',
  zip_code: '94043',
  city: 'Mountain View',
  state: 'CA',
  country: 'United States',
};

export const VALID_CARD = {
  credit_card_number: '4432801561520454',
  credit_card_expiration_month: '1',
  credit_card_expiration_year: String(new Date().getFullYear() + 1),
  credit_card_cvv: '672',
};

export const home = () => http.get(`${BASE_URL}/`);
export const product = (id) => http.get(`${BASE_URL}/product/${id}`);
export const cart = () => http.get(`${BASE_URL}/cart`);
export const emptyCart = () => http.post(`${BASE_URL}/cart/empty`);

export const addToCart = (productId, quantity, params) =>
  http.post(
    `${BASE_URL}/cart`,
    { product_id: productId, quantity: String(quantity) },
    params,
  );

export const setCurrency = (code) =>
  http.post(`${BASE_URL}/setCurrency`, { currency_code: code });

export const checkout = (card = VALID_CARD, address = ADDRESS) =>
  http.post(`${BASE_URL}/cart/checkout`, { ...address, ...card });

export const sessionCookie = () =>
  http.cookieJar().cookiesForURL(BASE_URL)['shop_session-id']?.[0];
